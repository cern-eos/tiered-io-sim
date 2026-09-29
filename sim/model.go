// Package sim sizes an NVMe / HDD / tape tier and applies the multi-stream
// HDD bandwidth formula across the whole spindle tier.
package sim

import (
	"math"
	"sort"
)

const (
	SecondsPerYear = 365 * 24 * 3600
	// RepackYears is how long a full library read-and-rewrite is spread over.
	RepackYears = 3
)

const (
	BindNone    = "none"
	BindNetwork = "network"
	BindDrives  = "drives"
	BindStream  = "stream"
	BindBoth    = "both"
)

// DriveStreamGBps is the effective bandwidth of one drive serving n concurrent
// streams. S is the contiguous segment between seeks, in GB. bwSeq is the
// drive's sequential bandwidth in GB/s. seekSec is the reposition time charged
// between streams.
//
//	BW(n) = n·S / (n·(S/BW_seq) + (n−1)·t_seek)    for n > 1
//
// Below one stream per drive the drive is under-subscribed, so bandwidth is
// n·BW_seq and seeks are not charged. Zero seek time keeps the drive at
// sequential bandwidth for any stream count at or above one.
func DriveStreamGBps(n, segmentGB, bwSeq, seekSec float64) float64 {
	if n <= 0 || segmentGB <= 0 || bwSeq <= 0 || math.IsNaN(n) || math.IsNaN(segmentGB) {
		return 0
	}
	if n <= 1 {
		return n * bwSeq
	}
	if seekSec <= 0 {
		return bwSeq
	}
	den := n*(segmentGB/bwSeq) + (n-1)*seekSec
	if den <= 0 {
		return 0
	}
	return (n * segmentGB) / den
}

// DriveAsymptoteGBps is the per-drive bandwidth as the stream count grows
// without bound: BW_seq·S / (S + BW_seq·t_seek).
func DriveAsymptoteGBps(segmentGB, bwSeq, seekSec float64) float64 {
	if segmentGB <= 0 || bwSeq <= 0 {
		return 0
	}
	if seekSec <= 0 {
		return bwSeq
	}
	return (bwSeq * segmentGB) / (segmentGB + bwSeq*seekSec)
}

// NodesForEB rounds a target capacity up to a whole node count.
func NodesForEB(targetEB float64, drivesPerNode int, driveTB float64) int {
	per := float64(drivesPerNode) * driveTB
	if per <= 0 || targetEB <= 0 || math.IsNaN(targetEB) {
		return 0
	}
	n := int(math.Round(targetEB * 1e6 / per))
	if n < 1 {
		return 1
	}
	return n
}

type NodeTier struct {
	Nodes         int     `json:"nodes"`
	NetworkGbps   float64 `json:"networkGbps"`
	DrivesPerNode int     `json:"drivesPerNode"`
	DriveSizeTB   float64 `json:"driveSizeTB"`
	DriveBWGBps   float64 `json:"driveBWGBps"`
	DriveIOPS     float64 `json:"driveIOPS"`
}

type TapeTier struct {
	Drives      int     `json:"drives"`
	DriveBWGBps float64 `json:"driveBWGBps"`
	CapacityEB  float64 `json:"capacityEB"`
}

type Stream struct {
	ReadStreams  float64 `json:"readStreams"`
	WriteStreams float64 `json:"writeStreams"`
	SegmentMB    float64 `json:"segmentMB"`
	SeekMs       float64 `json:"seekMs"`
}

type Workload struct {
	ComputeReadGBps  float64 `json:"computeReadGBps"`
	ComputeWriteGBps float64 `json:"computeWriteGBps"`
	NVMeHitRate      float64 `json:"nvmeHitRate"`
	// NVMeThroughFraction is the share of HDD client-read traffic staged
	// into the NVMe cache. The disks still perform that read.
	NVMeThroughFraction float64 `json:"nvmeThroughFraction"`
	// NVMeRereadFactor is how many times each staged byte is read from NVMe.
	// Egress is the staged rate times this factor, plus cache hits.
	NVMeRereadFactor     float64 `json:"nvmeRereadFactor"`
	WorkingSetPB         float64 `json:"workingSetPB"`
	ReserveFraction      float64 `json:"reserveFraction"`
	ArchiveEBPerYear     float64 `json:"archiveEBPerYear"`
	RecallGBps           float64 `json:"recallGBps"`
	BaselineEB           float64 `json:"baselineEB"`
	ObservedTBpsPerEB    float64 `json:"observedTBpsPerEB"`
	PrefetchHorizonHours float64 `json:"prefetchHorizonHours"`
	FileSizeGB           float64 `json:"fileSizeGB"`
}

type Prices struct {
	NodeCHF      float64 `json:"nodeCHF"`
	NVMeCHFPerTB float64 `json:"nvmeCHFPerTB"`
	HDDCHFPerTB  float64 `json:"hddCHFPerTB"`
	TapeDriveCHF float64 `json:"tapeDriveCHF"`
	TapeCHFPerTB float64 `json:"tapeCHFPerTB"`
}

// PowerRates are active watts. NodeW is the server without its drives.
type PowerRates struct {
	NodeW      float64 `json:"nodeW"`
	NVMeDriveW float64 `json:"nvmeDriveW"`
	HDDDriveW  float64 `json:"hddDriveW"`
	TapeDriveW float64 `json:"tapeDriveW"`
}

type Config struct {
	NVMe     NodeTier   `json:"nvme"`
	HDD      NodeTier   `json:"hdd"`
	Tape     TapeTier   `json:"tape"`
	Stream   Stream     `json:"stream"`
	Workload Workload   `json:"workload"`
	Prices   Prices     `json:"prices"`
	Power    PowerRates `json:"power"`
	// Hybrid installs the NVMe drives in the HDD nodes. Those drives add no
	// servers, and both tiers share the HDD network.
	Hybrid bool `json:"hybrid"`
	// Repack reads the tape library and writes it back over RepackYears,
	// in addition to the archive and recall rates.
	Repack bool `json:"repack"`
	// Layout is how the HDD tier protects data: "replica" or "ec10p2".
	Layout string `json:"layout"`
}

const (
	LayoutReplica = "replica"
	LayoutEC10p2  = "ec10p2"
)

// layoutFactors scales logical client IO into the bytes and disk streams
// the HDD tier actually serves.
type layoutFactors struct {
	ID           string
	Name         string
	ReadVolume   float64
	WriteVolume  float64
	ReadStreams  float64
	WriteStreams float64
	Note         string
}

func layoutOf(id string) layoutFactors {
	switch id {
	case LayoutEC10p2:
		return layoutFactors{
			ID:           LayoutEC10p2,
			Name:         "10+2 erasure coding",
			ReadVolume:   9.0 / 10.0,
			WriteVolume:  11.0 / 10.0,
			ReadStreams:  1,
			WriteStreams: 12,
			Note:         "10+2 erasure coding uses a gateway. Each logical write becomes 12 disk streams, and the gateway sends 11/10 of that volume onward. A read pulls 9/10 of the volume in. Archive is charged as a read and recall as a write.",
		}
	default:
		return layoutFactors{
			ID:           LayoutReplica,
			Name:         "2 replica",
			ReadVolume:   1,
			WriteVolume:  2,
			ReadStreams:  1,
			WriteStreams: 2,
			Note:         "2 replica writes both copies, so write traffic and write streams double. A read uses one copy, so read traffic and read streams stay at 1×. Archive is charged as a read and recall as a write.",
		}
	}
}

func DefaultConfig() Config {
	return Config{
		NVMe: NodeTier{
			Nodes: 48, NetworkGbps: 400, DrivesPerNode: 10,
			DriveSizeTB: 7.68, DriveBWGBps: 6, DriveIOPS: 1_000_000,
		},
		HDD: NodeTier{
			Nodes: 833, NetworkGbps: 100, DrivesPerNode: 90,
			DriveSizeTB: 20, DriveBWGBps: 0.27, DriveIOPS: 300,
		},
		Tape: TapeTier{Drives: 750, DriveBWGBps: 0.4, CapacityEB: 4},
		Stream: Stream{
			ReadStreams:  449820,
			WriteStreams: 149940,
			SegmentMB:    1,
			SeekMs:       7,
		},
		Workload: Workload{
			ComputeReadGBps:      1000,
			ComputeWriteGBps:     100,
			NVMeHitRate:          0,
			NVMeThroughFraction:  0.10,
			NVMeRereadFactor:     3,
			WorkingSetPB:         900,
			ReserveFraction:      0.10,
			ArchiveEBPerYear:     1,
			RecallGBps:           100,
			BaselineEB:           2.5,
			ObservedTBpsPerEB:    1,
			PrefetchHorizonHours: 24,
			FileSizeGB:           10,
		},
		Prices: Prices{
			NodeCHF:      10_000,
			NVMeCHFPerTB: 200,
			HDDCHFPerTB:  20,
			TapeDriveCHF: 25_000,
			TapeCHFPerTB: 10,
		},
		Power: PowerRates{
			NodeW:      300,
			NVMeDriveW: 20,
			HDDDriveW:  8,
			TapeDriveW: 30,
		},
		Layout: LayoutEC10p2,
	}
}

type TierStats struct {
	Nodes              int     `json:"nodes"`
	Drives             int     `json:"drives"`
	CapacityTB         float64 `json:"capacityTB"`
	CapacityEB         float64 `json:"capacityEB"`
	NetworkPerNodeGBps float64 `json:"networkPerNodeGBps"`
	NetworkGBps        float64 `json:"networkGBps"`
	DriveAggregateGBps float64 `json:"driveAggregateGBps"`
	DeliveredGBps      float64 `json:"deliveredGBps"`
	Binding            string  `json:"binding"`
	Limit              string  `json:"limit"`
	AggregateIOPS      float64 `json:"aggregateIOPS"`
	NetworkGbps        float64 `json:"networkGbps"`
}

type HDDStats struct {
	TierStats
	StreamPerDriveGBps     float64 `json:"streamPerDriveGBps"`
	StreamAggregateGBps    float64 `json:"streamAggregateGBps"`
	SequentialPerDriveGBps float64 `json:"sequentialPerDriveGBps"`
	Efficiency             float64 `json:"efficiency"`
	Degradation            float64 `json:"degradation"`
	StreamsPerDrive        float64 `json:"streamsPerDrive"`
	ReadStreams            float64 `json:"readStreams"`
	WriteStreams           float64 `json:"writeStreams"`
	LogicalReadStreams     float64 `json:"logicalReadStreams"`
	LogicalWriteStreams    float64 `json:"logicalWriteStreams"`
	ReadStreamFactor       float64 `json:"readStreamFactor"`
	WriteStreamFactor      float64 `json:"writeStreamFactor"`
	LayoutName             string  `json:"layoutName"`
	ReadDeliveredGBps      float64 `json:"readDeliveredGBps"`
	WriteDeliveredGBps     float64 `json:"writeDeliveredGBps"`
	FormulaIOPSPerDrive    float64 `json:"formulaIOPSPerDrive"`
	DeliveredIOPS          float64 `json:"deliveredIOPS"`
	ObservedGBps           float64 `json:"observedGBps"`
	ImpliedTBpsPerEB       float64 `json:"impliedTBpsPerEB"`
	BaselineEB             float64 `json:"baselineEB"`
	ReductionEB            float64 `json:"reductionEB"`
	ReductionFraction      float64 `json:"reductionFraction"`
	Mode                   string  `json:"mode"`
}

type TapeStats struct {
	Drives        int     `json:"drives"`
	PerDriveGBps  float64 `json:"perDriveGBps"`
	BandwidthGBps float64 `json:"bandwidthGBps"`
	CapacityEB    float64 `json:"capacityEB"`
	MaxEBPerYear  float64 `json:"maxEBPerYear"`
	MaxPBPerDay   float64 `json:"maxPBPerDay"`
	MaxPBPerHour  float64 `json:"maxPBPerHour"`
}

type Flow struct {
	ComputeReadGBps      float64 `json:"computeReadGBps"`
	ComputeWriteGBps     float64 `json:"computeWriteGBps"`
	NVMeHitRate          float64 `json:"nvmeHitRate"`
	NVMeThroughFraction  float64 `json:"nvmeThroughFraction"`
	NVMeRereadFactor     float64 `json:"nvmeRereadFactor"`
	PrefetchHours        float64 `json:"prefetchHours"`
	ArchiveGBps          float64 `json:"archiveGBps"`
	ArchiveFraction      float64 `json:"archiveFraction"`
	RecallGBps           float64 `json:"recallGBps"`
	RecallEBPerYear      float64 `json:"recallEBPerYear"`
	TapeDemandGBps       float64 `json:"tapeDemandGBps"`
	TapeSlackGBps        float64 `json:"tapeSlackGBps"`
	Repack               bool    `json:"repack"`
	RepackGBps           float64 `json:"repackGBps"`
	RecallPBPerHour      float64 `json:"recallPBPerHour"`
	RecallPBPerDay       float64 `json:"recallPBPerDay"`
	PrefetchPB           float64 `json:"prefetchPB"`
	NVMeHitDemandGBps    float64 `json:"nvmeHitDemandGBps"`
	NVMeServedGBps       float64 `json:"nvmeServedGBps"`
	NVMeThroughGBps      float64 `json:"nvmeThroughGBps"`
	NVMeEgressGBps       float64 `json:"nvmeEgressGBps"`
	Layout               string  `json:"layout"`
	LayoutName           string  `json:"layoutName"`
	LayoutNote           string  `json:"layoutNote"`
	ReadVolume           float64 `json:"readVolume"`
	WriteVolume          float64 `json:"writeVolume"`
	HDDComputeReadGBps   float64 `json:"hddComputeReadGBps"`
	HDDComputeWriteGBps  float64 `json:"hddComputeWriteGBps"`
	HDDArchiveGBps       float64 `json:"hddArchiveGBps"`
	HDDRecallGBps        float64 `json:"hddRecallGBps"`
	HDDRepackReadGBps    float64 `json:"hddRepackReadGBps"`
	HDDRepackWriteGBps   float64 `json:"hddRepackWriteGBps"`
	HDDDemandGBps        float64 `json:"hddDemandGBps"`
	HDDSlackGBps         float64 `json:"hddSlackGBps"`
	ObservedSlackGBps    float64 `json:"observedSlackGBps"`
	ObservedEnabled      bool    `json:"observedEnabled"`
	WorkingSetEB         float64 `json:"workingSetEB"`
	WorkingSetPB         float64 `json:"workingSetPB"`
	ReserveEB            float64 `json:"reserveEB"`
	ReserveFraction      float64 `json:"reserveFraction"`
	FreeEB               float64 `json:"freeEB"`
	FreePB               float64 `json:"freePB"`
	WorkingSetFits       bool    `json:"workingSetFits"`
	StageWorkingSetHours float64 `json:"stageWorkingSetHours"`
	TapeToHardware       float64 `json:"tapeToHardware"`
	TapeToObserved       float64 `json:"tapeToObserved"`
	ComputeDemandGBps    float64 `json:"computeDemandGBps"`
}

type Formula struct {
	Streams       float64  `json:"streams"`
	SegmentMB     float64  `json:"segmentMB"`
	SeekMs        float64  `json:"seekMs"`
	SeqMBps       float64  `json:"seqMBps"`
	PerDriveMBps  float64  `json:"perDriveMBps"`
	AsymptoteMBps float64  `json:"asymptoteMBps"`
	Drives        int      `json:"drives"`
	Nodes         int      `json:"nodes"`
	NetworkGbps   float64  `json:"networkGbps"`
	SpindleGBps   float64  `json:"spindleGBps"`
	NetworkGBps   float64  `json:"networkGBps"`
	DeliveredGBps float64  `json:"deliveredGBps"`
	Binding       string   `json:"binding"`
	Mode          string   `json:"mode"`
	Efficiency    float64  `json:"efficiency"`
	Lines         []string `json:"lines"`
}

type CurvePoint struct {
	StreamsPerDrive float64 `json:"streamsPerDrive"`
	TotalStreams    float64 `json:"totalStreams"`
	PerDriveGBps    float64 `json:"perDriveGBps"`
	SpindleGBps     float64 `json:"spindleGBps"`
	DeliveredGBps   float64 `json:"deliveredGBps"`
	ReadGBps        float64 `json:"readGBps"`
	WriteGBps       float64 `json:"writeGBps"`
	Degradation     float64 `json:"degradation"`
	NetworkGBps     float64 `json:"networkGBps"`
	ObservedGBps    float64 `json:"observedGBps"`
}

type SweepPoint struct {
	TargetEB          float64 `json:"targetEB"`
	Custom            bool    `json:"custom"`
	Active            bool    `json:"active"`
	Nodes             int     `json:"nodes"`
	CapacityEB        float64 `json:"capacityEB"`
	DeliveredGBps     float64 `json:"deliveredGBps"`
	ObservedGBps      float64 `json:"observedGBps"`
	ObservedEnabled   bool    `json:"observedEnabled"`
	TapeToHardware    float64 `json:"tapeToHardware"`
	TapeToObserved    float64 `json:"tapeToObserved"`
	HardwareSlackGBps float64 `json:"hardwareSlackGBps"`
	ObservedSlackGBps float64 `json:"observedSlackGBps"`
	WorkingSetFits    bool    `json:"workingSetFits"`
	NVMeCHF           float64 `json:"nvmeCHF"`
	HDDCHF            float64 `json:"hddCHF"`
	TapeCHF           float64 `json:"tapeCHF"`
	CostCHF           float64 `json:"costCHF"`
	PowerW            float64 `json:"powerW"`
}

type Knee struct {
	HardwareTargetEB float64 `json:"hardwareTargetEB"`
	HardwareOK       bool    `json:"hardwareOK"`
	ObservedTargetEB float64 `json:"observedTargetEB"`
	ObservedOK       bool    `json:"observedOK"`
}

type Bounds struct {
	CapacityFloorEB   float64 `json:"capacityFloorEB"`
	HardwareComputeEB float64 `json:"hardwareComputeEB"`
	HardwareComputeOK bool    `json:"hardwareComputeOK"`
	ObservedComputeEB float64 `json:"observedComputeEB"`
	ObservedComputeOK bool    `json:"observedComputeOK"`
	HardwareBudgetEB  float64 `json:"hardwareBudgetEB"`
	HardwareBudgetOK  bool    `json:"hardwareBudgetOK"`
	ObservedBudgetEB  float64 `json:"observedBudgetEB"`
	ObservedBudgetOK  bool    `json:"observedBudgetOK"`
	HardwareMinEB     float64 `json:"hardwareMinEB"`
	HardwareMinOK     bool    `json:"hardwareMinOK"`
	ObservedMinEB     float64 `json:"observedMinEB"`
	ObservedMinOK     bool    `json:"observedMinOK"`
	ObservedEnabled   bool    `json:"observedEnabled"`
	ObservedTBpsPerEB float64 `json:"observedTBpsPerEB"`
	ReserveFraction   float64 `json:"reserveFraction"`
}

type Verdict struct {
	Tone  string   `json:"tone"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

type Cost struct {
	NVMeNodesCHF   float64 `json:"nvmeNodesCHF"`
	NVMeMediaCHF   float64 `json:"nvmeMediaCHF"`
	HDDNodesCHF    float64 `json:"hddNodesCHF"`
	HDDMediaCHF    float64 `json:"hddMediaCHF"`
	TapeDrivesCHF  float64 `json:"tapeDrivesCHF"`
	TapeMediaCHF   float64 `json:"tapeMediaCHF"`
	NVMeCHF        float64 `json:"nvmeCHF"`
	HDDCHF         float64 `json:"hddCHF"`
	TapeCHF        float64 `json:"tapeCHF"`
	TotalCHF       float64 `json:"totalCHF"`
	BaselineNodes  int     `json:"baselineNodes"`
	BaselineHDDCHF float64 `json:"baselineHDDCHF"`
	HDDDeltaCHF    float64 `json:"hddDeltaCHF"`
}

type Power struct {
	NVMeNodesW  float64 `json:"nvmeNodesW"`
	NVMeDrivesW float64 `json:"nvmeDrivesW"`
	HDDNodesW   float64 `json:"hddNodesW"`
	HDDDrivesW  float64 `json:"hddDrivesW"`
	TapeDrivesW float64 `json:"tapeDrivesW"`
	NVMeW       float64 `json:"nvmeW"`
	HDDW        float64 `json:"hddW"`
	TapeW       float64 `json:"tapeW"`
	TotalW      float64 `json:"totalW"`
}

type Summary struct {
	NVMeIOPS              float64 `json:"nvmeIOPS"`
	HDDIOPS               float64 `json:"hddIOPS"`
	TotalIOPS             float64 `json:"totalIOPS"`
	CostCHF               float64 `json:"costCHF"`
	NVMeCapacityEB        float64 `json:"nvmeCapacityEB"`
	HDDCapacityEB         float64 `json:"hddCapacityEB"`
	TapeCapacityEB        float64 `json:"tapeCapacityEB"`
	TotalCapacityEB       float64 `json:"totalCapacityEB"`
	FileSizeGB            float64 `json:"fileSizeGB"`
	NVMeFilesPerSec       float64 `json:"nvmeFilesPerSec"`
	HDDReadFilesPerSec    float64 `json:"hddReadFilesPerSec"`
	HDDWriteFilesPerSec   float64 `json:"hddWriteFilesPerSec"`
	HDDFilesPerSec        float64 `json:"hddFilesPerSec"`
	TapeFilesPerSec       float64 `json:"tapeFilesPerSec"`
	NVMeBandwidthGBps     float64 `json:"nvmeBandwidthGBps"`
	HDDReadBandwidthGBps  float64 `json:"hddReadBandwidthGBps"`
	HDDWriteBandwidthGBps float64 `json:"hddWriteBandwidthGBps"`
	HDDBandwidthGBps      float64 `json:"hddBandwidthGBps"`
	TapeBandwidthGBps     float64 `json:"tapeBandwidthGBps"`
	UsableBandwidthGBps   float64 `json:"usableBandwidthGBps"`
	HDDUsableIOPS         float64 `json:"hddUsableIOPS"`
	UsableIOPS            float64 `json:"usableIOPS"`
}

type Result struct {
	Hybrid   bool         `json:"hybrid"`
	Repack   bool         `json:"repack"`
	NVMe     TierStats    `json:"nvme"`
	HDD      HDDStats     `json:"hdd"`
	Tape     TapeStats    `json:"tape"`
	Flow     Flow         `json:"flow"`
	Formula  Formula      `json:"formula"`
	Curve    []CurvePoint `json:"curve"`
	Sweep    []SweepPoint `json:"sweep"`
	Knee     Knee         `json:"knee"`
	Bounds   Bounds       `json:"bounds"`
	Cost     Cost         `json:"cost"`
	Power    Power        `json:"power"`
	Summary  Summary      `json:"summary"`
	Verdict  Verdict      `json:"verdict"`
	Warnings []string     `json:"warnings"`
}

func Evaluate(cfg Config) Result {
	warnings := sanitize(&cfg)
	r := evaluate(cfg)
	r.Sweep = sweep(cfg)
	r.Knee = kneeFrom(r.Sweep)
	r.Verdict = buildVerdict(r)
	r.Warnings = warnings
	return r
}

func evaluate(cfg Config) Result {
	nvme := evalNode(cfg.NVMe)
	hdd := evalHDD(cfg.HDD, cfg.Stream, cfg.Workload, cfg.Layout)
	if cfg.Hybrid {
		applyHybrid(cfg, &nvme, &hdd)
	}
	assignDeliveredIOPS(&hdd, cfg.Stream.SegmentMB)
	tape := evalTape(cfg.Tape)
	flow := evalFlow(cfg.Workload, cfg.Layout, cfg.Repack, nvme, hdd, tape)
	bounds := evalBounds(hdd, flow, cfg.Workload)
	formula := evalFormula(cfg, hdd)
	curveNet := hdd.NetworkGBps
	if cfg.Hybrid {
		remain := hdd.NetworkGBps - flow.NVMeServedGBps
		if remain < 0 {
			remain = 0
		}
		curveNet = remain
		formula.Lines = append(formula.Lines, hybridLine(hdd.NetworkGBps, flow.NVMeServedGBps))
	}
	cost := evalCost(cfg, nvme, hdd, tape)
	summary := evalSummary(nvme, hdd, tape, cost, cfg.Workload.FileSizeGB, flow.ReadVolume, flow.WriteVolume, cfg.Stream.SegmentMB, cfg.Hybrid)
	if cfg.Workload.FileSizeGB > 0 {
		formula.Lines = append(formula.Lines, "A "+fmtNum(cfg.Workload.FileSizeGB)+" GB file gives the HDD tier "+fmtNum(summary.HDDReadFilesPerSec)+" read files/s and "+fmtNum(summary.HDDWriteFilesPerSec)+" write files/s at this stream mix.")
	}
	if cfg.Repack {
		formula.Lines = append(formula.Lines, repackLine(tape.CapacityEB, flow))
	}
	if flow.NVMeThroughFraction > 0 {
		formula.Lines = append(formula.Lines, throughLine(flow))
	}
	return Result{
		Hybrid:  cfg.Hybrid,
		Repack:  cfg.Repack,
		NVMe:    nvme,
		HDD:     hdd,
		Tape:    tape,
		Flow:    flow,
		Bounds:  bounds,
		Cost:    cost,
		Power:   evalPower(cfg.Power, nvme, hdd, tape),
		Summary: summary,
		Formula: formula,
		Curve:   evalCurve(cfg, hdd.CapacityEB, curveNet),
	}
}

func assignDeliveredIOPS(hdd *HDDStats, segmentMB float64) {
	if segmentMB <= 0 || hdd.DeliveredGBps <= 0 {
		hdd.DeliveredIOPS = 0
		return
	}
	hdd.DeliveredIOPS = hdd.DeliveredGBps / (segmentMB / 1000)
}

func filesPerSec(gbps, fileGB, volume float64) float64 {
	if fileGB <= 0 || gbps <= 0 || volume <= 0 {
		return 0
	}
	return gbps / (fileGB * volume)
}

func evalSummary(nvme TierStats, hdd HDDStats, tape TapeStats, cost Cost, fileGB, readVolume, writeVolume, segmentMB float64, hybrid bool) Summary {
	hddReadBW := 0.0
	if readVolume > 0 {
		hddReadBW = hdd.ReadDeliveredGBps / readVolume
	}
	hddWriteBW := 0.0
	if writeVolume > 0 {
		hddWriteBW = hdd.WriteDeliveredGBps / writeVolume
	}
	s := Summary{
		NVMeIOPS:              nvme.AggregateIOPS,
		HDDIOPS:               hdd.DeliveredIOPS,
		CostCHF:               cost.TotalCHF,
		NVMeCapacityEB:        nvme.CapacityEB,
		HDDCapacityEB:         hdd.CapacityEB,
		TapeCapacityEB:        tape.CapacityEB,
		FileSizeGB:            fileGB,
		NVMeFilesPerSec:       filesPerSec(nvme.DeliveredGBps, fileGB, 1),
		HDDReadFilesPerSec:    filesPerSec(hdd.ReadDeliveredGBps, fileGB, readVolume),
		HDDWriteFilesPerSec:   filesPerSec(hdd.WriteDeliveredGBps, fileGB, writeVolume),
		TapeFilesPerSec:       filesPerSec(tape.BandwidthGBps, fileGB, 1),
		NVMeBandwidthGBps:     nvme.DeliveredGBps,
		HDDReadBandwidthGBps:  hddReadBW,
		HDDWriteBandwidthGBps: hddWriteBW,
		HDDBandwidthGBps:      hdd.DeliveredGBps,
		TapeBandwidthGBps:     tape.BandwidthGBps,
	}
	// Each tier contributes its delivered rate, already the minimum of drives
	// and network. The HDD rate is one pipe: a read excludes a write.
	s.UsableBandwidthGBps = s.NVMeBandwidthGBps + s.HDDBandwidthGBps
	if hybrid && hdd.NetworkGBps > 0 && s.UsableBandwidthGBps > hdd.NetworkGBps {
		s.UsableBandwidthGBps = hdd.NetworkGBps
	}
	if segmentMB > 0 {
		s.HDDUsableIOPS = s.HDDBandwidthGBps / (segmentMB / 1000)
	}
	s.UsableIOPS = s.NVMeIOPS + s.HDDUsableIOPS
	s.TotalIOPS = s.NVMeIOPS + s.HDDIOPS
	s.TotalCapacityEB = s.NVMeCapacityEB + s.HDDCapacityEB + s.TapeCapacityEB
	s.HDDFilesPerSec = s.HDDReadFilesPerSec + s.HDDWriteFilesPerSec
	return s
}

// applyHybrid places the NVMe complement in the HDD nodes. Drive count is
// HDD nodes times the NVMe drives-per-node setting. The separate NVMe node
// count and network are not used. NVMe traffic is served first and the HDD
// tier keeps what remains of the shared network.
func applyHybrid(cfg Config, nvme *TierStats, hdd *HDDStats) {
	drives := cfg.HDD.Nodes * cfg.NVMe.DrivesPerNode
	capTB := float64(drives) * cfg.NVMe.DriveSizeTB
	driveAgg := float64(drives) * cfg.NVMe.DriveBWGBps
	net := hdd.NetworkGBps
	nvmeDel, nvmeBind := clipBandwidth(driveAgg, net, BindDrives, BindNetwork)
	*nvme = TierStats{
		Nodes:              0,
		Drives:             drives,
		CapacityTB:         capTB,
		CapacityEB:         capTB / 1e6,
		NetworkPerNodeGBps: hdd.NetworkPerNodeGBps,
		NetworkGBps:        net,
		DriveAggregateGBps: driveAgg,
		DeliveredGBps:      nvmeDel,
		Binding:            nvmeBind,
		Limit:              hybridNVMeLimit(nvmeBind),
		AggregateIOPS:      float64(drives) * cfg.NVMe.DriveIOPS,
		NetworkGbps:        cfg.HDD.NetworkGbps,
	}
	served := cfg.Workload.ComputeReadGBps * cfg.Workload.NVMeHitRate
	if served > nvmeDel {
		served = nvmeDel
	}
	remain := net - served
	if remain < 0 {
		remain = 0
	}
	hddDel, hddBind := clipBandwidth(hdd.StreamAggregateGBps, remain, BindStream, BindNetwork)
	hdd.DeliveredGBps = hddDel
	hdd.Binding = hddBind
	if served > 0 && (hddBind == BindNetwork || hddBind == BindBoth) {
		hdd.Limit = "the shared HDD network after NVMe traffic"
	} else {
		hdd.Limit = limitClause(hddBind, hdd.Mode)
	}
	hdd.ReadDeliveredGBps, hdd.WriteDeliveredGBps = splitBandwidth(hdd.ReadStreams, hdd.WriteStreams, hddDel)
	if hdd.CapacityEB > 0 {
		hdd.ImpliedTBpsPerEB = (hddDel / 1000) / hdd.CapacityEB
	}
}

func hybridNVMeLimit(binding string) string {
	switch binding {
	case BindNetwork:
		return "the shared HDD network"
	case BindBoth:
		return "drive bandwidth and the shared HDD network"
	default:
		return limitClause(binding, "")
	}
}

func hybridLine(network, served float64) string {
	remain := network - served
	if remain < 0 {
		remain = 0
	}
	return "Hybrid layout installs the NVMe drives in the HDD nodes. They add no servers and share the " + fmtBW(network) + " network. NVMe traffic uses " + fmtBW(served) + ", leaving " + fmtBW(remain) + " for the HDD tier."
}

func evalCost(cfg Config, nvme TierStats, hdd HDDStats, tape TapeStats) Cost {
	p := cfg.Prices
	c := Cost{
		NVMeNodesCHF:  float64(nvme.Nodes) * p.NodeCHF,
		NVMeMediaCHF:  nvme.CapacityTB * p.NVMeCHFPerTB,
		HDDNodesCHF:   float64(hdd.Nodes) * p.NodeCHF,
		HDDMediaCHF:   hdd.CapacityTB * p.HDDCHFPerTB,
		TapeDrivesCHF: float64(tape.Drives) * p.TapeDriveCHF,
		TapeMediaCHF:  tape.CapacityEB * 1e6 * p.TapeCHFPerTB,
	}
	c.NVMeCHF = c.NVMeNodesCHF + c.NVMeMediaCHF
	c.HDDCHF = c.HDDNodesCHF + c.HDDMediaCHF
	c.TapeCHF = c.TapeDrivesCHF + c.TapeMediaCHF
	c.TotalCHF = c.NVMeCHF + c.HDDCHF + c.TapeCHF
	if cfg.Workload.BaselineEB > 0 {
		nodes := NodesForEB(cfg.Workload.BaselineEB, cfg.HDD.DrivesPerNode, cfg.HDD.DriveSizeTB)
		capTB := float64(nodes*cfg.HDD.DrivesPerNode) * cfg.HDD.DriveSizeTB
		c.BaselineNodes = nodes
		c.BaselineHDDCHF = float64(nodes)*p.NodeCHF + capTB*p.HDDCHFPerTB
		c.HDDDeltaCHF = c.HDDCHF - c.BaselineHDDCHF
	}
	return c
}

func evalPower(rates PowerRates, nvme TierStats, hdd HDDStats, tape TapeStats) Power {
	p := Power{
		NVMeNodesW:  float64(nvme.Nodes) * rates.NodeW,
		NVMeDrivesW: float64(nvme.Drives) * rates.NVMeDriveW,
		HDDNodesW:   float64(hdd.Nodes) * rates.NodeW,
		HDDDrivesW:  float64(hdd.Drives) * rates.HDDDriveW,
		TapeDrivesW: float64(tape.Drives) * rates.TapeDriveW,
	}
	p.NVMeW = p.NVMeNodesW + p.NVMeDrivesW
	p.HDDW = p.HDDNodesW + p.HDDDrivesW
	p.TapeW = p.TapeDrivesW
	p.TotalW = p.NVMeW + p.HDDW + p.TapeW
	return p
}

func evalNode(t NodeTier) TierStats {
	drives := t.Nodes * t.DrivesPerNode
	if t.Nodes < 0 || t.DrivesPerNode < 0 {
		drives = 0
	}
	capTB := float64(drives) * t.DriveSizeTB
	netPer := t.NetworkGbps / 8
	net := float64(t.Nodes) * netPer
	driveAgg := float64(drives) * t.DriveBWGBps
	delivered, binding := clipBandwidth(driveAgg, net, BindDrives, BindNetwork)
	return TierStats{
		Nodes:              t.Nodes,
		Drives:             drives,
		CapacityTB:         capTB,
		CapacityEB:         capTB / 1e6,
		NetworkPerNodeGBps: netPer,
		NetworkGBps:        net,
		DriveAggregateGBps: driveAgg,
		DeliveredGBps:      delivered,
		Binding:            binding,
		Limit:              limitClause(binding, ""),
		AggregateIOPS:      float64(drives) * t.DriveIOPS,
		NetworkGbps:        t.NetworkGbps,
	}
}

func streamsPerDrive(s Stream, drives int) float64 {
	if drives <= 0 {
		return 0
	}
	return (s.ReadStreams + s.WriteStreams) / float64(drives)
}

func splitBandwidth(readStreams, writeStreams, delivered float64) (readBW, writeBW float64) {
	total := readStreams + writeStreams
	if total <= 0 || delivered <= 0 {
		return 0, 0
	}
	readBW = delivered * readStreams / total
	return readBW, delivered - readBW
}

func seekDegradation(n, perDrive, sequential float64) float64 {
	if n <= 1 || sequential <= 0 || perDrive <= 0 {
		return 0
	}
	lost := 1 - perDrive/sequential
	if lost < 0 {
		return 0
	}
	if lost > 1 {
		return 1
	}
	return lost
}

func evalHDD(t NodeTier, s Stream, wl Workload, layout string) HDDStats {
	base := evalNode(t)
	factors := layoutOf(layout)
	readStreams := s.ReadStreams * factors.ReadStreams
	writeStreams := s.WriteStreams * factors.WriteStreams
	n := streamsPerDrive(Stream{ReadStreams: readStreams, WriteStreams: writeStreams}, base.Drives)
	mode := streamMode(n, s.SeekMs/1000)
	per := DriveStreamGBps(n, s.SegmentMB/1000, t.DriveBWGBps, s.SeekMs/1000)
	agg := float64(base.Drives) * per
	delivered, binding := clipBandwidth(agg, base.NetworkGBps, BindStream, BindNetwork)
	base.DeliveredGBps = delivered
	base.Binding = binding
	base.Limit = limitClause(binding, mode)
	eff := 0.0
	if t.DriveBWGBps > 0 {
		eff = per / t.DriveBWGBps
	}
	readBW, writeBW := splitBandwidth(readStreams, writeStreams, delivered)
	formulaIOPS := 0.0
	if s.SegmentMB > 0 {
		formulaIOPS = per / (s.SegmentMB / 1000)
	}
	observed := 0.0
	if wl.ObservedTBpsPerEB > 0 {
		observed = base.CapacityEB * wl.ObservedTBpsPerEB * 1000
	}
	implied := 0.0
	if base.CapacityEB > 0 {
		implied = (delivered / 1000) / base.CapacityEB
	}
	reduction := wl.BaselineEB - base.CapacityEB
	frac := 0.0
	if wl.BaselineEB > 0 {
		frac = reduction / wl.BaselineEB
	}
	return HDDStats{
		TierStats:              base,
		StreamPerDriveGBps:     per,
		StreamAggregateGBps:    agg,
		SequentialPerDriveGBps: t.DriveBWGBps,
		Efficiency:             eff,
		Degradation:            seekDegradation(n, per, t.DriveBWGBps),
		StreamsPerDrive:        n,
		ReadStreams:            readStreams,
		WriteStreams:           writeStreams,
		LogicalReadStreams:     s.ReadStreams,
		LogicalWriteStreams:    s.WriteStreams,
		ReadStreamFactor:       factors.ReadStreams,
		WriteStreamFactor:      factors.WriteStreams,
		LayoutName:             factors.Name,
		ReadDeliveredGBps:      readBW,
		WriteDeliveredGBps:     writeBW,
		FormulaIOPSPerDrive:    formulaIOPS,
		ObservedGBps:           observed,
		ImpliedTBpsPerEB:       implied,
		BaselineEB:             wl.BaselineEB,
		ReductionEB:            reduction,
		ReductionFraction:      frac,
		Mode:                   mode,
	}
}

func evalTape(t TapeTier) TapeStats {
	bw := float64(t.Drives) * t.DriveBWGBps
	return TapeStats{
		Drives:        t.Drives,
		PerDriveGBps:  t.DriveBWGBps,
		BandwidthGBps: bw,
		CapacityEB:    t.CapacityEB,
		MaxEBPerYear:  gbpsToEBPerYear(bw),
		MaxPBPerDay:   gbpsToPBPerDay(bw),
		MaxPBPerHour:  gbpsToPBPerHour(bw),
	}
}

func evalFlow(wl Workload, layout string, repack bool, nvme TierStats, hdd HDDStats, tape TapeStats) Flow {
	factors := layoutOf(layout)
	archive := ebPerYearToGBps(wl.ArchiveEBPerYear)
	recall := wl.RecallGBps
	hitDemand := wl.ComputeReadGBps * wl.NVMeHitRate
	served := math.Min(hitDemand, nvme.DeliveredGBps)
	hddRead := wl.ComputeReadGBps - served
	if hddRead < 0 {
		hddRead = 0
	}
	hddReadTraffic := hddRead * factors.ReadVolume
	hddWriteTraffic := wl.ComputeWriteGBps * factors.WriteVolume
	archiveTraffic := archive * factors.ReadVolume
	recallTraffic := recall * factors.WriteVolume
	repackEach := 0.0
	if repack {
		repackEach = ebPerYearToGBps(tape.CapacityEB / RepackYears)
	}
	// A repack reads the library onto disk, then reads that data back off
	// disk to write the new tapes. Archive is a disk read; recall is a disk write.
	hddRepackRead := repackEach * factors.ReadVolume
	hddRepackWrite := repackEach * factors.WriteVolume
	remain := nvme.DeliveredGBps - served
	if remain < 0 {
		remain = 0
	}
	through := hddReadTraffic * wl.NVMeThroughFraction
	factor := wl.NVMeRereadFactor
	if factor < 0 || math.IsNaN(factor) {
		factor = 0
	}
	if factor > 0 {
		if through*factor > remain {
			through = remain / factor
		}
	} else if through > remain {
		through = remain
	}
	egress := served + through*factor
	if egress > nvme.DeliveredGBps {
		egress = nvme.DeliveredGBps
	}
	demand := hddReadTraffic + hddWriteTraffic + archiveTraffic + recallTraffic + hddRepackRead + hddRepackWrite
	tapeDemand := archive + recall + 2*repackEach
	workingSetEB := wl.WorkingSetPB / 1000
	reserveEB := hdd.CapacityEB * wl.ReserveFraction
	freeEB := hdd.CapacityEB - workingSetEB - reserveEB
	usable := hdd.CapacityEB * (1 - wl.ReserveFraction)
	fits := workingSetEB <= usable+1e-9
	observedEnabled := wl.ObservedTBpsPerEB > 0
	observedSlack := 0.0
	if observedEnabled {
		observedSlack = hdd.ObservedGBps - demand
	}
	pbPerHour := gbpsToPBPerHour(recall)
	stageHours := 0.0
	if pbPerHour > 0 {
		stageHours = wl.WorkingSetPB / pbPerHour
	}
	archFrac := 0.0
	if tape.BandwidthGBps > 0 {
		archFrac = archive / tape.BandwidthGBps
	}
	toHW := 0.0
	if hdd.DeliveredGBps > 0 {
		toHW = tape.BandwidthGBps / hdd.DeliveredGBps
	}
	toObs := 0.0
	if hdd.ObservedGBps > 0 {
		toObs = tape.BandwidthGBps / hdd.ObservedGBps
	}
	return Flow{
		ComputeReadGBps:      wl.ComputeReadGBps,
		ComputeWriteGBps:     wl.ComputeWriteGBps,
		NVMeHitRate:          wl.NVMeHitRate,
		NVMeThroughFraction:  wl.NVMeThroughFraction,
		NVMeRereadFactor:     factor,
		PrefetchHours:        wl.PrefetchHorizonHours,
		ArchiveGBps:          archive,
		ArchiveFraction:      archFrac,
		RecallGBps:           recall,
		RecallEBPerYear:      gbpsToEBPerYear(recall),
		TapeDemandGBps:       tapeDemand,
		TapeSlackGBps:        tape.BandwidthGBps - tapeDemand,
		Repack:               repack,
		RepackGBps:           repackEach,
		RecallPBPerHour:      pbPerHour,
		RecallPBPerDay:       gbpsToPBPerDay(recall),
		PrefetchPB:           pbPerHour * wl.PrefetchHorizonHours,
		Layout:               factors.ID,
		LayoutName:           factors.Name,
		LayoutNote:           factors.Note,
		ReadVolume:           factors.ReadVolume,
		WriteVolume:          factors.WriteVolume,
		NVMeHitDemandGBps:    hitDemand,
		NVMeServedGBps:       served,
		NVMeThroughGBps:      through,
		NVMeEgressGBps:       egress,
		HDDComputeReadGBps:   hddReadTraffic,
		HDDComputeWriteGBps:  hddWriteTraffic,
		HDDArchiveGBps:       archiveTraffic,
		HDDRecallGBps:        recallTraffic,
		HDDRepackReadGBps:    hddRepackRead,
		HDDRepackWriteGBps:   hddRepackWrite,
		HDDDemandGBps:        demand,
		HDDSlackGBps:         hdd.DeliveredGBps - demand,
		ObservedSlackGBps:    observedSlack,
		ObservedEnabled:      observedEnabled,
		WorkingSetEB:         workingSetEB,
		WorkingSetPB:         wl.WorkingSetPB,
		ReserveEB:            reserveEB,
		ReserveFraction:      wl.ReserveFraction,
		FreeEB:               freeEB,
		FreePB:               freeEB * 1000,
		WorkingSetFits:       fits,
		StageWorkingSetHours: stageHours,
		TapeToHardware:       toHW,
		TapeToObserved:       toObs,
		ComputeDemandGBps:    hddReadTraffic + hddWriteTraffic,
	}
}

func evalBounds(hdd HDDStats, flow Flow, wl Workload) Bounds {
	floor := 0.0
	if wl.ReserveFraction < 1 {
		floor = flow.WorkingSetEB / (1 - wl.ReserveFraction)
	}
	hwCompute, hwComputeOK := scaleFloor(hdd.CapacityEB, hdd.DeliveredGBps, flow.ComputeDemandGBps)
	hwBudget, hwBudgetOK := scaleFloor(hdd.CapacityEB, hdd.DeliveredGBps, flow.HDDDemandGBps)
	obsCompute, obsComputeOK := 0.0, false
	obsBudget, obsBudgetOK := 0.0, false
	if wl.ObservedTBpsPerEB > 0 {
		per := wl.ObservedTBpsPerEB * 1000
		obsCompute, obsComputeOK = scaleFloor(1, per, flow.ComputeDemandGBps)
		obsBudget, obsBudgetOK = scaleFloor(1, per, flow.HDDDemandGBps)
	}
	hwMin, hwMinOK := combineFloor(floor, hwBudget, hwBudgetOK, flow.HDDDemandGBps)
	obsMin, obsMinOK := 0.0, false
	if wl.ObservedTBpsPerEB > 0 {
		obsMin, obsMinOK = combineFloor(floor, obsBudget, obsBudgetOK, flow.HDDDemandGBps)
	}
	return Bounds{
		CapacityFloorEB:   floor,
		HardwareComputeEB: hwCompute,
		HardwareComputeOK: hwComputeOK,
		ObservedComputeEB: obsCompute,
		ObservedComputeOK: obsComputeOK,
		HardwareBudgetEB:  hwBudget,
		HardwareBudgetOK:  hwBudgetOK,
		ObservedBudgetEB:  obsBudget,
		ObservedBudgetOK:  obsBudgetOK,
		HardwareMinEB:     hwMin,
		HardwareMinOK:     hwMinOK,
		ObservedMinEB:     obsMin,
		ObservedMinOK:     obsMinOK,
		ObservedEnabled:   wl.ObservedTBpsPerEB > 0,
		ObservedTBpsPerEB: wl.ObservedTBpsPerEB,
		ReserveFraction:   wl.ReserveFraction,
	}
}

func scaleFloor(capacityEB, delivered, demand float64) (float64, bool) {
	if demand <= 0 {
		return 0, true
	}
	if delivered <= 0 || capacityEB <= 0 {
		return 0, false
	}
	return capacityEB * demand / delivered, true
}

func combineFloor(capacityFloor, budget float64, budgetOK bool, demand float64) (float64, bool) {
	if demand > 0 && !budgetOK {
		return 0, false
	}
	if !budgetOK {
		return capacityFloor, true
	}
	return math.Max(capacityFloor, budget), true
}

func evalFormula(cfg Config, hdd HDDStats) Formula {
	s := cfg.Stream
	mode := hdd.Mode
	asymMBps := DriveAsymptoteGBps(s.SegmentMB/1000, cfg.HDD.DriveBWGBps, s.SeekMs/1000) * 1000
	f := Formula{
		Streams:       hdd.StreamsPerDrive,
		SegmentMB:     s.SegmentMB,
		SeekMs:        s.SeekMs,
		SeqMBps:       cfg.HDD.DriveBWGBps * 1000,
		PerDriveMBps:  hdd.StreamPerDriveGBps * 1000,
		AsymptoteMBps: asymMBps,
		Drives:        hdd.Drives,
		Nodes:         hdd.Nodes,
		NetworkGbps:   cfg.HDD.NetworkGbps,
		SpindleGBps:   hdd.StreamAggregateGBps,
		NetworkGBps:   hdd.NetworkGBps,
		DeliveredGBps: hdd.DeliveredGBps,
		Binding:       hdd.Binding,
		Mode:          mode,
		Efficiency:    hdd.Efficiency,
	}
	f.Lines = formulaLines(f, hdd)
	return f
}

func formulaLines(f Formula, hdd HDDStats) []string {
	if f.Mode == "idle" || f.SeqMBps <= 0 || f.SegmentMB <= 0 {
		return []string{"Segment size, sequential bandwidth, or stream count is zero, so the stream model delivers nothing."}
	}
	var lines []string
	switch f.Mode {
	case "partial":
		lines = append(lines, "n = "+fmtNum(f.Streams)+" is below one stream per drive, so BW = "+fmtNum(f.Streams)+" × "+fmtMBps(f.SeqMBps)+" = "+fmtMBps(f.PerDriveMBps)+" per drive. Seeks are not charged.")
	case "sequential":
		lines = append(lines, "n = "+fmtNum(f.Streams)+", so each active drive streams at "+fmtMBps(f.SeqMBps)+". Seeks are not charged.")
	default:
		lines = append(lines,
			"BW("+fmtNum(f.Streams)+") = "+fmtNum(f.Streams)+" × "+fmtMB(f.SegmentMB)+
				" / ("+fmtNum(f.Streams)+" × ("+fmtMB(f.SegmentMB)+" / "+fmtMBps(f.SeqMBps)+") + ("+fmtNum(f.Streams)+" − 1) × "+fmtNum(f.SeekMs)+" ms) = "+
				fmtMBps(f.PerDriveMBps)+" per drive, "+fmtPct(f.Efficiency)+" of sequential.")
	}
	lines = append(lines, layoutStreamLine(hdd))
	if hdd.Degradation >= 0.0005 {
		lines = append(lines, "Seek contention removes "+fmtPct(hdd.Degradation)+" of sequential bandwidth on each drive.")
	}
	lines = append(lines,
		fmtInt(f.Drives)+" drives produce "+fmtBW(f.SpindleGBps)+". The "+fmtInt(f.Nodes)+"-node "+fmtNum(f.NetworkGbps)+" GbE network allows "+fmtBW(f.NetworkGBps)+". Delivered bandwidth is "+fmtBW(f.DeliveredGBps)+", limited by "+hdd.Limit+".")
	if f.Mode == "contended" {
		lines = append(lines, "As the stream count grows, one drive approaches "+fmtMBps(f.AsymptoteMBps)+".")
	}
	if f.SegmentMB > 0 {
		per := 0.0
		if hdd.Drives > 0 {
			per = hdd.DeliveredIOPS / float64(hdd.Drives)
		}
		lines = append(lines, "HDD IOPS are delivered bandwidth divided by the segment size: "+fmtInt(int(math.Round(hdd.DeliveredIOPS)))+" ("+fmtNum(per)+" / drive).")
	}
	if hdd.CapacityEB > 0 {
		line := "Across this capacity the hardware model implies " + fmtNum(hdd.ImpliedTBpsPerEB) + " TB/s per EB."
		if hdd.ObservedGBps > 0 {
			line += " Observed scaling assigns " + fmtBW(hdd.ObservedGBps) + "."
		}
		lines = append(lines, line)
	}
	return lines
}

func repackLine(capacityEB float64, flow Flow) string {
	return "A " + fmtInt(RepackYears) + "-year repack reads and rewrites the " + fmtEB(capacityEB) +
		" archive, adding " + fmtBW(flow.RepackGBps) + " of tape read and " + fmtBW(flow.RepackGBps) +
		" of tape write. The HDD tier reads " + fmtBW(flow.HDDRepackReadGBps) +
		" to feed the rewrite and writes " + fmtBW(flow.HDDRepackWriteGBps) + " to take the tape read."
}

func throughLine(flow Flow) string {
	return fmtPct(flow.NVMeThroughFraction) + " of HDD client reads are staged into NVMe at " +
		fmtBW(flow.NVMeThroughGBps) + ". A re-read factor of " + fmtNum(flow.NVMeRereadFactor) +
		" raises NVMe egress to " + fmtBW(flow.NVMeEgressGBps) + ". The disks still perform the staged read."
}

func layoutStreamLine(hdd HDDStats) string {
	disk := fmtInt(int(math.Round(hdd.ReadStreams))) + " disk read streams and " + fmtInt(int(math.Round(hdd.WriteStreams))) + " disk write streams average " + fmtNum(hdd.StreamsPerDrive) + " per drive. Read streams get " + fmtBW(hdd.ReadDeliveredGBps) + " and write streams get " + fmtBW(hdd.WriteDeliveredGBps) + "."
	if hdd.WriteStreamFactor == 1 && hdd.ReadStreamFactor == 1 {
		return disk
	}
	name := hdd.LayoutName
	if name == "" {
		name = "The layout"
	}
	return name + " turns " + fmtInt(int(math.Round(hdd.LogicalWriteStreams))) + " logical write streams into " + fmtInt(int(math.Round(hdd.WriteStreams))) + " disk write streams. " + disk
}

func evalCurve(cfg Config, capacityEB, networkGBps float64) []CurvePoint {
	drives := cfg.HDD.Nodes * cfg.HDD.DrivesPerNode
	factors := layoutOf(cfg.Layout)
	readStreams := cfg.Stream.ReadStreams * factors.ReadStreams
	writeStreams := cfg.Stream.WriteStreams * factors.WriteStreams
	op := streamsPerDrive(Stream{ReadStreams: readStreams, WriteStreams: writeStreams}, drives)
	n := op
	if n < 1 {
		n = 1
	}
	hi := math.Max(64, n)
	xs := make([]float64, 0, 70)
	for i := 0; i <= 63; i++ {
		xs = append(xs, 1+(hi-1)*float64(i)/63)
	}
	xs = appendUnique(xs, op)
	sort.Float64s(xs)
	net := networkGBps
	observed := 0.0
	if cfg.Workload.ObservedTBpsPerEB > 0 {
		observed = capacityEB * cfg.Workload.ObservedTBpsPerEB * 1000
	}
	seg := cfg.Stream.SegmentMB / 1000
	seek := cfg.Stream.SeekMs / 1000
	pts := make([]CurvePoint, 0, len(xs))
	for _, x := range xs {
		if x < 0 {
			continue
		}
		per := DriveStreamGBps(x, seg, cfg.HDD.DriveBWGBps, seek)
		spindle := float64(drives) * per
		delivered, _ := clipBandwidth(spindle, net, BindStream, BindNetwork)
		readBW, writeBW := splitBandwidth(readStreams, writeStreams, delivered)
		pts = append(pts, CurvePoint{
			StreamsPerDrive: x,
			TotalStreams:    x * float64(drives),
			PerDriveGBps:    per,
			SpindleGBps:     spindle,
			DeliveredGBps:   delivered,
			ReadGBps:        readBW,
			WriteGBps:       writeBW,
			Degradation:     seekDegradation(x, per, cfg.HDD.DriveBWGBps),
			NetworkGBps:     net,
			ObservedGBps:    observed,
		})
	}
	return pts
}

var sweepTargets = []float64{2.5, 2, 1.5, 1.25, 1, 0.75, 0.5}

func sweep(cfg Config) []SweepPoint {
	pts := make([]SweepPoint, 0, len(sweepTargets)+1)
	seen := map[int]bool{}
	for _, target := range sweepTargets {
		nodes := NodesForEB(target, cfg.HDD.DrivesPerNode, cfg.HDD.DriveSizeTB)
		if nodes == 0 || seen[nodes] {
			continue
		}
		seen[nodes] = true
		c := cfg
		c.HDD.Nodes = nodes
		st := evaluate(c)
		pts = append(pts, makeSweep(target, false, cfg.HDD.Nodes == nodes, st))
	}
	if cfg.HDD.Nodes > 0 && !seen[cfg.HDD.Nodes] {
		st := evaluate(cfg)
		pts = append(pts, makeSweep(st.HDD.CapacityEB, true, true, st))
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].CapacityEB > pts[j].CapacityEB })
	return pts
}

func makeSweep(target float64, custom, active bool, st Result) SweepPoint {
	return SweepPoint{
		TargetEB:          target,
		Custom:            custom,
		Active:            active,
		Nodes:             st.HDD.Nodes,
		CapacityEB:        st.HDD.CapacityEB,
		DeliveredGBps:     st.HDD.DeliveredGBps,
		ObservedGBps:      st.HDD.ObservedGBps,
		ObservedEnabled:   st.Flow.ObservedEnabled,
		TapeToHardware:    st.Flow.TapeToHardware,
		TapeToObserved:    st.Flow.TapeToObserved,
		HardwareSlackGBps: st.Flow.HDDSlackGBps,
		ObservedSlackGBps: st.Flow.ObservedSlackGBps,
		WorkingSetFits:    st.Flow.WorkingSetFits,
		NVMeCHF:           st.Cost.NVMeCHF,
		HDDCHF:            st.Cost.HDDCHF,
		TapeCHF:           st.Cost.TapeCHF,
		CostCHF:           st.Cost.TotalCHF,
		PowerW:            st.Power.TotalW,
	}
}

func kneeFrom(pts []SweepPoint) Knee {
	hw, hwOK := smallestCovering(pts, false)
	obs, obsOK := smallestCovering(pts, true)
	return Knee{HardwareTargetEB: hw, HardwareOK: hwOK, ObservedTargetEB: obs, ObservedOK: obsOK}
}

func smallestCovering(pts []SweepPoint, observed bool) (float64, bool) {
	if observed {
		enabled := false
		for _, p := range pts {
			if !p.Custom && p.ObservedEnabled {
				enabled = true
				break
			}
		}
		if !enabled {
			return 0, false
		}
	}
	best := math.Inf(1)
	target := 0.0
	found := false
	for _, p := range pts {
		if p.Custom {
			continue
		}
		slack := p.HardwareSlackGBps
		if observed {
			slack = p.ObservedSlackGBps
		}
		if p.WorkingSetFits && slack >= -1e-4 && p.CapacityEB < best {
			best = p.CapacityEB
			target = p.TargetEB
			found = true
		}
	}
	return target, found
}

func clipBandwidth(primary, network float64, primaryName, networkName string) (float64, string) {
	switch {
	case primary <= 0 && network <= 0:
		return 0, BindNone
	case primary <= 0:
		return 0, primaryName
	case network <= 0:
		return 0, networkName
	}
	diff := math.Abs(primary - network)
	if diff <= 1e-6*math.Max(1, math.Max(primary, network)) {
		return primary, BindBoth
	}
	if primary < network {
		return primary, primaryName
	}
	return network, networkName
}

func streamMode(n, seekSec float64) string {
	switch {
	case n <= 0:
		return "idle"
	case n < 1:
		return "partial"
	case seekSec <= 0 || n <= 1:
		return "sequential"
	default:
		return "contended"
	}
}

func limitClause(binding, mode string) string {
	switch binding {
	case BindNetwork:
		return "the node network"
	case BindBoth:
		if mode == "contended" {
			return "seek contention and the node network"
		}
		return "drive bandwidth and the node network"
	case BindNone:
		return "an empty tier"
	case BindDrives:
		return "drive bandwidth"
	case BindStream:
		switch mode {
		case "partial":
			return "having fewer streams than drives"
		case "sequential":
			return "sequential drive bandwidth"
		default:
			return "seek contention"
		}
	default:
		return binding
	}
}

func gbpsToPBPerDay(gbps float64) float64 {
	return gbps * 86400 / 1e6
}

func gbpsToPBPerHour(gbps float64) float64 {
	return gbps * 3600 / 1e6
}

func gbpsToEBPerYear(gbps float64) float64 {
	return gbps * SecondsPerYear / 1e9
}

func ebPerYearToGBps(ebPerYear float64) float64 {
	return ebPerYear * 1e9 / SecondsPerYear
}

func appendUnique(xs []float64, v float64) []float64 {
	for _, x := range xs {
		if math.Abs(x-v) < 1e-9 {
			return xs
		}
	}
	return append(xs, v)
}

func sanitize(cfg *Config) []string {
	var w []string
	sanitizeTier("NVMe", &cfg.NVMe, &w)
	sanitizeTier("HDD", &cfg.HDD, &w)
	cfg.Tape.Drives = nonnegInt("Tape drives", cfg.Tape.Drives, &w)
	cfg.Tape.DriveBWGBps = nonnegF("Tape bandwidth", cfg.Tape.DriveBWGBps, &w)
	cfg.Tape.CapacityEB = nonnegF("Tape capacity", cfg.Tape.CapacityEB, &w)
	cfg.Stream.ReadStreams = nonnegF("Read streams", cfg.Stream.ReadStreams, &w)
	cfg.Stream.WriteStreams = nonnegF("Write streams", cfg.Stream.WriteStreams, &w)
	cfg.Stream.SegmentMB = nonnegF("Segment size", cfg.Stream.SegmentMB, &w)
	cfg.Stream.SeekMs = nonnegF("Seek time", cfg.Stream.SeekMs, &w)
	cfg.Workload.ComputeReadGBps = nonnegF("Compute read", cfg.Workload.ComputeReadGBps, &w)
	cfg.Workload.ComputeWriteGBps = nonnegF("Compute write", cfg.Workload.ComputeWriteGBps, &w)
	cfg.Workload.WorkingSetPB = nonnegF("Working set", cfg.Workload.WorkingSetPB, &w)
	cfg.Workload.ArchiveEBPerYear = nonnegF("Archive rate", cfg.Workload.ArchiveEBPerYear, &w)
	cfg.Workload.RecallGBps = nonnegF("Recall bandwidth", cfg.Workload.RecallGBps, &w)
	cfg.Workload.BaselineEB = nonnegF("Baseline HDD", cfg.Workload.BaselineEB, &w)
	cfg.Workload.ObservedTBpsPerEB = nonnegF("Observed scaling", cfg.Workload.ObservedTBpsPerEB, &w)
	cfg.Workload.PrefetchHorizonHours = nonnegF("Prefetch horizon", cfg.Workload.PrefetchHorizonHours, &w)
	cfg.Workload.FileSizeGB = nonnegF("File size", cfg.Workload.FileSizeGB, &w)
	cfg.Prices.NodeCHF = nonnegF("Node price", cfg.Prices.NodeCHF, &w)
	cfg.Prices.NVMeCHFPerTB = nonnegF("NVMe media price", cfg.Prices.NVMeCHFPerTB, &w)
	cfg.Prices.HDDCHFPerTB = nonnegF("HDD media price", cfg.Prices.HDDCHFPerTB, &w)
	cfg.Prices.TapeDriveCHF = nonnegF("Tape drive price", cfg.Prices.TapeDriveCHF, &w)
	cfg.Prices.TapeCHFPerTB = nonnegF("Tape media price", cfg.Prices.TapeCHFPerTB, &w)
	cfg.Power.NodeW = nonnegF("Node power", cfg.Power.NodeW, &w)
	cfg.Power.NVMeDriveW = nonnegF("NVMe drive power", cfg.Power.NVMeDriveW, &w)
	cfg.Power.HDDDriveW = nonnegF("HDD power", cfg.Power.HDDDriveW, &w)
	cfg.Power.TapeDriveW = nonnegF("Tape drive power", cfg.Power.TapeDriveW, &w)
	switch cfg.Layout {
	case "", LayoutEC10p2:
		cfg.Layout = LayoutEC10p2
	case LayoutReplica:
	default:
		w = append(w, "HDD layout was unrecognized and was treated as 10+2 erasure coding.")
		cfg.Layout = LayoutEC10p2
	}
	if cfg.Workload.NVMeHitRate < 0 || cfg.Workload.NVMeHitRate > 1 || math.IsNaN(cfg.Workload.NVMeHitRate) {
		w = append(w, "NVMe hit rate was outside 0–100% and was clamped.")
		cfg.Workload.NVMeHitRate = clamp01(cfg.Workload.NVMeHitRate)
	}
	if cfg.Workload.NVMeThroughFraction < 0 || cfg.Workload.NVMeThroughFraction > 1 || math.IsNaN(cfg.Workload.NVMeThroughFraction) {
		w = append(w, "NVMe read-through was outside 0–100% and was clamped.")
		cfg.Workload.NVMeThroughFraction = clamp01(cfg.Workload.NVMeThroughFraction)
	}
	if cfg.Workload.NVMeRereadFactor < 0 || math.IsNaN(cfg.Workload.NVMeRereadFactor) {
		w = append(w, "NVMe re-read factor was negative and was treated as zero.")
		cfg.Workload.NVMeRereadFactor = 0
	}
	if cfg.Workload.ReserveFraction < 0 || math.IsNaN(cfg.Workload.ReserveFraction) {
		w = append(w, "Reserve fraction was invalid and was treated as zero.")
		cfg.Workload.ReserveFraction = 0
	}
	if cfg.Workload.ReserveFraction >= 1 {
		w = append(w, "Reserve was at least 100% and was treated as 99% so a capacity floor can be computed.")
		cfg.Workload.ReserveFraction = 0.99
	}
	return w
}

func sanitizeTier(name string, t *NodeTier, w *[]string) {
	t.Nodes = nonnegInt(name+" nodes", t.Nodes, w)
	t.DrivesPerNode = nonnegInt(name+" drives per node", t.DrivesPerNode, w)
	t.NetworkGbps = nonnegF(name+" network", t.NetworkGbps, w)
	t.DriveSizeTB = nonnegF(name+" drive size", t.DriveSizeTB, w)
	t.DriveBWGBps = nonnegF(name+" drive bandwidth", t.DriveBWGBps, w)
	t.DriveIOPS = nonnegF(name+" drive IOPS", t.DriveIOPS, w)
}

func nonnegF(name string, v float64, w *[]string) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		*w = append(*w, name+" was invalid and was treated as zero.")
		return 0
	}
	return v
}

func nonnegInt(name string, v int, w *[]string) int {
	if v < 0 {
		*w = append(*w, name+" was negative and was treated as zero.")
		return 0
	}
	return v
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
