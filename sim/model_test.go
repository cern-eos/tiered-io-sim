package sim

import (
	"math"
	"strings"
	"testing"
)

func near(t *testing.T, got, want, tol float64, name string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %g, want %g (tol %g)", name, got, want, tol)
	}
}

func TestDriveStreamEdges(t *testing.T) {
	if got := DriveStreamGBps(1, 0.001, 0.28, 0.007); got != 0.28 {
		t.Fatalf("n=1: got %g", got)
	}
	if got := DriveStreamGBps(0.5, 0.001, 0.28, 0.007); got != 0.14 {
		t.Fatalf("n=0.5: got %g", got)
	}
	if got := DriveStreamGBps(8, 0.001, 0.28, 0); got != 0.28 {
		t.Fatalf("zero seek: got %g", got)
	}
	if got := DriveStreamGBps(0, 0.001, 0.28, 0.007); got != 0 {
		t.Fatalf("n=0: got %g", got)
	}
	s := 1.0 / 1000
	bw := 0.28
	seek := 7.0 / 1000
	want := (8 * s) / (8*(s/bw) + 7*seek)
	near(t, DriveStreamGBps(8, s, bw, seek), want, 1e-12, "n=8")
	asym := (bw * s) / (s + bw*seek)
	near(t, DriveAsymptoteGBps(s, bw, seek), asym, 1e-12, "asymptote")
	near(t, DriveStreamGBps(1e6, s, bw, seek), asym, 1e-4, "large n")
}

func TestCalendarUnits(t *testing.T) {
	near(t, gbpsToPBPerDay(300), 25.92, 1e-9, "pb/day")
	near(t, gbpsToPBPerHour(300), 1.08, 1e-9, "pb/hour")
	near(t, gbpsToEBPerYear(300), 9.4608, 1e-9, "eb/year")
	near(t, ebPerYearToGBps(1), 1e9/float64(SecondsPerYear), 1e-12, "archive")
	if got := fmtBW(ebPerYearToGBps(1)); got != "31.7 GB/s" {
		t.Fatalf("archive format %q", got)
	}
	if got := fmtPct(ebPerYearToGBps(1) / 300); got != "10.6%" {
		t.Fatalf("archive percent %q", got)
	}
}

func TestDefaultScenario(t *testing.T) {
	r := Evaluate(DefaultConfig())
	if r.HDD.Binding != BindStream {
		t.Fatalf("binding %s, want stream", r.HDD.Binding)
	}
	if r.NVMe.Binding != BindNetwork {
		t.Fatalf("nvme binding %s", r.NVMe.Binding)
	}
	near(t, r.NVMe.DeliveredGBps, 2400, 1e-9, "nvme bw")
	near(t, r.Tape.BandwidthGBps, 300, 1e-12, "tape")
	near(t, r.HDD.CapacityEB, 1.4994, 1e-9, "hdd eb")
	near(t, float64(r.HDD.Drives), 74970, 0, "drives")
	near(t, r.HDD.NetworkGBps, 10412.5, 1e-9, "net")
	near(t, r.HDD.DeliveredGBps, 7594.645, 1e-2, "delivered")
	near(t, r.HDD.StreamsPerDrive, 10, 1e-9, "streams per drive")
	near(t, r.HDD.WriteStreams, 2*149940, 1e-6, "disk writes")
	near(t, r.HDD.ReadDeliveredGBps, 0.6*r.HDD.DeliveredGBps, 1e-6, "read share")
	near(t, r.Flow.WriteVolume, 2, 1e-12, "write volume")
	near(t, r.Flow.HDDComputeWriteGBps, 200, 1e-9, "write traffic")
	near(t, r.Flow.HDDRecallGBps, 200, 1e-9, "recall traffic")
	near(t, r.HDD.DeliveredIOPS, r.HDD.DeliveredGBps/0.001, 1, "hdd iops")
	near(t, r.Summary.TotalIOPS, 480*1e6+r.HDD.DeliveredIOPS, 1, "total iops")
	near(t, r.Summary.CostCHF, r.Cost.TotalCHF, 1e-6, "summary cost")
	near(t, r.Summary.TotalCapacityEB, r.NVMe.CapacityEB+r.HDD.CapacityEB+4, 1e-9, "summary capacity")
	near(t, r.Summary.FileSizeGB, 10, 1e-12, "file size")
	near(t, r.Summary.NVMeFilesPerSec, 2400/10, 1e-9, "nvme files")
	near(t, r.Summary.TapeFilesPerSec, 300/10, 1e-9, "tape files")
	near(t, r.Summary.HDDReadFilesPerSec, r.HDD.ReadDeliveredGBps/10, 1e-6, "hdd read files")
	near(t, r.Summary.HDDWriteFilesPerSec, r.HDD.WriteDeliveredGBps/(10*2), 1e-6, "hdd write files")
	near(t, r.Summary.HDDFilesPerSec, r.Summary.HDDReadFilesPerSec+r.Summary.HDDWriteFilesPerSec, 1e-9, "hdd files")
	near(t, r.HDD.Degradation, 1-r.HDD.Efficiency, 1e-9, "degradation")
	near(t, r.Flow.ArchiveGBps, 1e9/float64(SecondsPerYear), 1e-9, "archive")
	near(t, r.Bounds.ObservedMinEB, 1.4317, 5e-4, "obs min")
	near(t, r.Bounds.HardwareMinEB, 1, 1e-9, "hw min")
	if !r.Knee.HardwareOK || r.Knee.HardwareTargetEB != 1 {
		t.Fatalf("hardware knee %+v", r.Knee)
	}
	if !r.Knee.ObservedOK || r.Knee.ObservedTargetEB != 1.5 {
		t.Fatalf("observed knee %+v", r.Knee)
	}
	if r.Verdict.Tone != "tight" || r.Verdict.Title != "Thin margin" {
		t.Fatalf("verdict %+v", r.Verdict)
	}
	text := strings.Join(r.Verdict.Lines, " ")
	for _, phrase := range []string{"1.5 EB", "40%", "7.59 TB/s", "10.6%", "1.43 EB", "1 EB", "5.07 TB/s per EB", "seek contention"} {
		if !strings.Contains(text, phrase) {
			t.Fatalf("verdict missing %q\n%s", phrase, text)
		}
	}
	joined := strings.Join(r.Formula.Lines, "\n")
	for _, phrase := range []string{"94.6 MB/s", "disk write streams", "2 replica"} {
		if !strings.Contains(joined, phrase) {
			t.Fatalf("formula missing %q\n%s", phrase, joined)
		}
	}
	if r.Curve[0].Degradation != 0 {
		t.Fatalf("one stream should not degrade: %+v", r.Curve[0])
	}
	prevDeg := 0.0
	for _, p := range r.Curve {
		if p.Degradation+1e-12 < prevDeg {
			t.Fatalf("degradation fell at n=%g", p.StreamsPerDrive)
		}
		prevDeg = p.Degradation
	}
	if len(r.Sweep) != 7 {
		t.Fatalf("sweep len %d", len(r.Sweep))
	}
	var prev float64 = 1e9
	for _, p := range r.Sweep {
		if p.CapacityEB > prev {
			t.Fatalf("sweep not descending")
		}
		prev = p.CapacityEB
	}
}

func TestSequentialBindsOnNetwork(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Stream.ReadStreams = float64(cfg.HDD.Nodes * cfg.HDD.DrivesPerNode)
	cfg.Stream.WriteStreams = 0
	r := Evaluate(cfg)
	if r.HDD.Binding != BindNetwork {
		t.Fatalf("binding %s", r.HDD.Binding)
	}
	near(t, r.HDD.DeliveredGBps, 10412.5, 1e-9, "delivered")
	near(t, r.HDD.Degradation, 0, 1e-12, "degradation")
	near(t, r.HDD.WriteDeliveredGBps, 0, 1e-12, "write")
	near(t, r.HDD.ReadDeliveredGBps, r.HDD.DeliveredGBps, 1e-9, "read")
}

func TestHitRateOverflow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workload.NVMeHitRate = 1
	cfg.Workload.ComputeReadGBps = 3000
	r := Evaluate(cfg)
	near(t, r.Flow.NVMeServedGBps, 2400, 1e-6, "served")
	near(t, r.Flow.HDDComputeReadGBps, 600, 1e-6, "hdd read")
}

func TestSmallHDDOversubscribed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HDD.Nodes = NodesForEB(0.5, cfg.HDD.DrivesPerNode, cfg.HDD.DriveSizeTB)
	r := Evaluate(cfg)
	if r.Flow.WorkingSetFits {
		t.Fatal("expected working set to miss")
	}
	if r.Verdict.Tone != "over" {
		t.Fatalf("tone %s (%s)", r.Verdict.Tone, r.Verdict.Title)
	}
}

func TestDefaultCost(t *testing.T) {
	r := Evaluate(DefaultConfig())
	near(t, r.Cost.NVMeNodesCHF, 480_000, 1e-6, "nvme nodes")
	near(t, r.Cost.NVMeMediaCHF, 3686.4*200, 1e-6, "nvme media")
	near(t, r.Cost.HDDNodesCHF, 8_330_000, 1e-6, "hdd nodes")
	near(t, r.Cost.HDDMediaCHF, 1_499_400*20, 1e-6, "hdd media")
	near(t, r.Cost.TapeDrivesCHF, 750*25_000, 1e-6, "tape drives")
	near(t, r.Cost.TapeMediaCHF, 40_000_000, 1e-6, "tape media")
	near(t, r.Cost.TotalCHF, 98_285_280, 1e-3, "total")
	near(t, r.Cost.BaselineHDDCHF, 63_894_000, 1e-3, "baseline hdd")
	near(t, r.Cost.HDDDeltaCHF, 38_318_000-63_894_000, 1e-3, "hdd delta")
	if r.Cost.BaselineNodes != 1389 {
		t.Fatalf("baseline nodes %d", r.Cost.BaselineNodes)
	}
	if got := fmtCHF(r.Cost.TotalCHF); got != "CHF 98.29 million" {
		t.Fatalf("total format %q", got)
	}
	active := 0
	for _, p := range r.Sweep {
		if p.Active {
			active++
			near(t, p.NVMeCHF, r.Cost.NVMeCHF, 1e-6, "sweep nvme")
			near(t, p.HDDCHF, r.Cost.HDDCHF, 1e-6, "sweep hdd")
			near(t, p.TapeCHF, r.Cost.TapeCHF, 1e-6, "sweep tape")
			near(t, p.CostCHF, r.Cost.TotalCHF, 1e-6, "sweep cost")
		}
	}
	if active != 1 {
		t.Fatalf("active sweep rows %d", active)
	}

	cfg := DefaultConfig()
	cfg.Prices.NodeCHF = -5
	r = Evaluate(cfg)
	if r.Cost.NVMeNodesCHF != 0 || len(r.Warnings) == 0 {
		t.Fatalf("negative price not clamped: %+v %v", r.Cost, r.Warnings)
	}
}

func TestDefaultPower(t *testing.T) {
	r := Evaluate(DefaultConfig())
	near(t, r.Power.NVMeNodesW, 48*300, 1e-9, "nvme nodes")
	near(t, r.Power.NVMeDrivesW, 480*20, 1e-9, "nvme drives")
	near(t, r.Power.HDDNodesW, 833*300, 1e-9, "hdd nodes")
	near(t, r.Power.HDDDrivesW, 74970*8, 1e-9, "hdd drives")
	near(t, r.Power.TapeDrivesW, 750*30, 1e-9, "tape drives")
	near(t, r.Power.TotalW, 896_160, 1e-6, "total")
	if got := fmtPower(r.Power.TotalW); got != "896.2 kW" {
		t.Fatalf("power format %q", got)
	}
	for _, p := range r.Sweep {
		if p.Active {
			near(t, p.PowerW, r.Power.TotalW, 1e-6, "sweep power")
		}
	}
}

func TestHybridSharesHDDNetwork(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Hybrid = true
	r := Evaluate(cfg)
	if !r.Hybrid {
		t.Fatal("hybrid flag")
	}
	if r.NVMe.Nodes != 0 {
		t.Fatalf("nvme servers %d", r.NVMe.Nodes)
	}
	if r.NVMe.Drives != 833*10 {
		t.Fatalf("nvme drives %d", r.NVMe.Drives)
	}
	if r.NVMe.Binding != BindNetwork {
		t.Fatalf("nvme binding %s", r.NVMe.Binding)
	}
	near(t, r.NVMe.DeliveredGBps, 10412.5, 1e-6, "nvme on shared net")
	near(t, r.NVMe.NetworkGBps, r.HDD.NetworkGBps, 1e-9, "same network")
	near(t, r.HDD.DeliveredGBps, 7594.645, 1e-2, "idle cache leaves hdd")
	if r.HDD.Binding != BindStream {
		t.Fatalf("hdd binding %s", r.HDD.Binding)
	}
	if r.Cost.NVMeNodesCHF != 0 || r.Power.NVMeNodesW != 0 {
		t.Fatalf("hybrid still charges servers: cost %g power %g", r.Cost.NVMeNodesCHF, r.Power.NVMeNodesW)
	}
	near(t, r.Cost.NVMeMediaCHF, float64(833*10)*7.68*200, 1e-3, "nvme media")
	near(t, r.Power.NVMeDrivesW, float64(833*10)*20, 1e-6, "nvme drive power")
	near(t, r.Power.HDDNodesW, 833*300, 1e-6, "hdd nodes stay")
	joined := strings.Join(r.Formula.Lines, " ")
	if !strings.Contains(joined, "Hybrid layout") || !strings.Contains(joined, "add no servers") {
		t.Fatalf("formula %v", r.Formula.Lines)
	}

	cfg.Workload.NVMeHitRate = 1
	cfg.Workload.ComputeReadGBps = 3000
	r = Evaluate(cfg)
	near(t, r.Flow.NVMeServedGBps, 3000, 1e-6, "served")
	near(t, r.HDD.DeliveredGBps, 10412.5-3000, 1e-3, "hdd after nvme")
	if r.HDD.Binding != BindNetwork {
		t.Fatalf("hdd binding %s", r.HDD.Binding)
	}
	if r.HDD.Limit != "the shared HDD network after NVMe traffic" {
		t.Fatalf("limit %q", r.HDD.Limit)
	}
	if r.Curve[0].NetworkGBps != 10412.5-3000 {
		t.Fatalf("curve network %g", r.Curve[0].NetworkGBps)
	}
}

func TestErasureCodingLayout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Layout = LayoutEC10p2
	r := Evaluate(cfg)
	if r.Flow.Layout != LayoutEC10p2 {
		t.Fatalf("layout %s", r.Flow.Layout)
	}
	near(t, r.Flow.ReadVolume, 0.9, 1e-12, "read volume")
	near(t, r.Flow.WriteVolume, 1.1, 1e-12, "write volume")
	near(t, r.HDD.WriteStreamFactor, 12, 1e-12, "write streams")
	near(t, r.HDD.WriteStreams, 12*149940, 1e-6, "disk writes")
	near(t, r.HDD.ReadStreams, 449820, 1e-6, "disk reads")
	near(t, r.HDD.StreamsPerDrive, 30, 1e-9, "n")
	near(t, r.Flow.HDDComputeReadGBps, 900, 1e-6, "read traffic")
	near(t, r.Flow.HDDComputeWriteGBps, 110, 1e-6, "write traffic")
	near(t, r.Flow.HDDRecallGBps, 110, 1e-6, "recall traffic")
	near(t, r.Flow.HDDArchiveGBps, 0.9*r.Flow.ArchiveGBps, 1e-9, "archive traffic")
	near(t, r.Flow.TapeDemandGBps, r.Flow.ArchiveGBps+r.Flow.RecallGBps, 1e-9, "tape stays logical")
	if !strings.Contains(strings.Join(r.Formula.Lines, " "), "10+2 erasure coding turns") {
		t.Fatalf("formula %v", r.Formula.Lines)
	}
}

func TestSanitizeNegative(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HDD.Nodes = -4
	cfg.Workload.NVMeHitRate = 3
	r := Evaluate(cfg)
	if r.HDD.Nodes != 0 {
		t.Fatalf("nodes %d", r.HDD.Nodes)
	}
	if r.Flow.NVMeHitRate != 1 {
		t.Fatalf("hit %g", r.Flow.NVMeHitRate)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
}
