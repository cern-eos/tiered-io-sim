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
	near(t, r.HDD.DeliveredGBps, 7731.712707, 1e-3, "delivered")
	near(t, r.HDD.StreamsPerDrive, 8, 1e-9, "streams per drive")
	near(t, r.HDD.ReadDeliveredGBps, 0.75*r.HDD.DeliveredGBps, 1e-6, "read share")
	near(t, r.HDD.Degradation, 1-r.HDD.Efficiency, 1e-9, "degradation")
	near(t, r.Flow.ArchiveGBps, 1e9/float64(SecondsPerYear), 1e-9, "archive")
	near(t, r.Bounds.ObservedMinEB, 1.2317, 5e-4, "obs min")
	near(t, r.Bounds.HardwareMinEB, 1, 1e-9, "hw min")
	if !r.Knee.HardwareOK || r.Knee.HardwareTargetEB != 1 {
		t.Fatalf("hardware knee %+v", r.Knee)
	}
	if !r.Knee.ObservedOK || r.Knee.ObservedTargetEB != 1.25 {
		t.Fatalf("observed knee %+v", r.Knee)
	}
	if r.Verdict.Tone != "ok" || r.Verdict.Title != "Within budget" {
		t.Fatalf("verdict %+v", r.Verdict)
	}
	text := strings.Join(r.Verdict.Lines, " ")
	for _, phrase := range []string{"1.5 EB", "40%", "7.73 TB/s", "10.6%", "1.23 EB", "1.25 EB", "1 EB", "5.16 TB/s per EB", "seek contention"} {
		if !strings.Contains(text, phrase) {
			t.Fatalf("verdict missing %q\n%s", phrase, text)
		}
	}
	joined := strings.Join(r.Formula.Lines, "\n")
	for _, phrase := range []string{"103.1 MB/s", "36.8%", "94.6 MB/s", "63.2%", "read streams"} {
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
