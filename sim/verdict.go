package sim

import "strings"

func buildVerdict(r Result) Verdict {
	v := Verdict{Lines: make([]string, 0, 6)}
	v.Tone, v.Title = toneOf(r)
	v.Lines = append(v.Lines, capacityLine(r))
	v.Lines = append(v.Lines, bandwidthLine(r))
	v.Lines = append(v.Lines, tapeLine(r))
	v.Lines = append(v.Lines, workingSetLine(r))
	if r.Flow.NVMeHitRate > 0 {
		v.Lines = append(v.Lines, nvmeLine(r))
	}
	v.Lines = append(v.Lines, minimumLine(r))
	return v
}

func toneOf(r Result) (string, string) {
	over := !r.Flow.WorkingSetFits || r.Flow.HDDSlackGBps < -0.05 || r.Flow.TapeSlackGBps < -0.05
	if over {
		return "over", "Oversubscribed"
	}
	if r.Flow.ObservedEnabled && r.Flow.ObservedSlackGBps < -0.05 {
		return "tight", "Below the observed line"
	}
	thin := false
	if r.HDD.DeliveredGBps > 0 && r.Flow.HDDSlackGBps >= 0 && r.Flow.HDDSlackGBps < 0.15*r.HDD.DeliveredGBps {
		thin = true
	}
	if r.Flow.ObservedEnabled && r.HDD.ObservedGBps > 0 && r.Flow.ObservedSlackGBps >= 0 && r.Flow.ObservedSlackGBps < 0.15*r.HDD.ObservedGBps {
		thin = true
	}
	if r.Tape.BandwidthGBps > 0 && r.Flow.TapeSlackGBps >= 0 && r.Flow.TapeSlackGBps < 0.15*r.Tape.BandwidthGBps {
		thin = true
	}
	if r.Flow.TapeToObserved > 0.35 {
		thin = true
	}
	if thin {
		return "tight", "Thin margin"
	}
	return "ok", "Within budget"
}

func capacityLine(r Result) string {
	line := "HDD capacity is " + fmtEB(r.HDD.CapacityEB)
	if r.HDD.BaselineEB <= 0 {
		return line + "."
	}
	if r.HDD.ReductionEB >= 0 {
		return line + ", " + fmtPct(r.HDD.ReductionFraction) + " under the " + fmtEB(r.HDD.BaselineEB) + " baseline, a reduction of " + fmtEB(r.HDD.ReductionEB) + "."
	}
	return line + ", " + fmtPct(-r.HDD.ReductionFraction) + " over the " + fmtEB(r.HDD.BaselineEB) + " baseline."
}

func bandwidthLine(r Result) string {
	line := "Delivered HDD bandwidth is " + fmtBW(r.HDD.DeliveredGBps) + ", limited by " + r.HDD.Limit +
		". Sequential spindles would provide " + fmtBW(r.HDD.DriveAggregateGBps) +
		" and the node network allows " + fmtBW(r.HDD.NetworkGBps) + "."
	if r.Flow.ObservedEnabled {
		line += " Observed scaling assigns " + fmtBW(r.HDD.ObservedGBps) +
			". The hardware model implies " + fmtNum(r.HDD.ImpliedTBpsPerEB) + " TB/s per EB."
		line += " Tape at " + fmtBW(r.Tape.BandwidthGBps) + " is " + fmtPct(r.Flow.TapeToHardware) +
			" of delivered hardware bandwidth and " + fmtPct(r.Flow.TapeToObserved) + " of observed bandwidth."
		return line
	}
	line += " Tape at " + fmtBW(r.Tape.BandwidthGBps) + " is " + fmtPct(r.Flow.TapeToHardware) + " of delivered hardware bandwidth."
	return line
}

func tapeLine(r Result) string {
	line := "Mandatory archive of " + fmtEB(r.Flow.ArchiveGBps*float64(SecondsPerYear)/1e9) + "/year averages " +
		fmtBW(r.Flow.ArchiveGBps)
	if r.Tape.BandwidthGBps > 0 {
		line += ", " + fmtPct(r.Flow.ArchiveFraction) + " of tape"
	}
	line += ". Recall at " + fmtBW(r.Flow.RecallGBps) + " moves " + fmtEB(r.Flow.RecallEBPerYear) +
		"/year and stages " + fmtPB(r.Flow.RecallPBPerDay) + "/day."
	what := "Archive and recall"
	if r.Flow.Repack {
		line += " A " + fmtInt(RepackYears) + "-year repack reads and rewrites the " + fmtEB(r.Tape.CapacityEB) +
			" library at " + fmtBW(r.Flow.RepackGBps) + " each way."
		what = "Archive, recall, and repack"
	}
	if r.Flow.TapeSlackGBps >= 0 {
		line += " " + what + " together use " + fmtBW(r.Flow.TapeDemandGBps) + " and leave " + fmtBW(r.Flow.TapeSlackGBps) + " of tape."
	} else {
		line += " " + what + " together require " + fmtBW(r.Flow.TapeDemandGBps) + ", " + fmtBW(-r.Flow.TapeSlackGBps) + " above tape bandwidth."
	}
	return line
}

func workingSetLine(r Result) string {
	reserve := fmtPct(r.Flow.ReserveFraction)
	if !r.Flow.WorkingSetFits {
		return "The " + fmtPB(r.Flow.WorkingSetPB) + " working set plus a " + reserve + " reserve needs " +
			fmtEB(r.Bounds.CapacityFloorEB) + ". This tier holds " + fmtEB(r.HDD.CapacityEB) + "."
	}
	line := "The " + fmtPB(r.Flow.WorkingSetPB) + " working set fits inside " + fmtEB(r.HDD.CapacityEB) +
		" alongside a " + reserve + " reserve, leaving " + fmtPB(r.Flow.FreePB) + " free."
	if r.Flow.PrefetchHours <= 0 {
		return line
	}
	if r.Flow.RecallGBps <= 0 {
		return line + " Recall bandwidth is zero, so the prefetch horizon stages nothing."
	}
	staged := fmtPB(r.Flow.PrefetchPB)
	window := trimFloat(r.Flow.PrefetchHours, 1)
	if r.Flow.PrefetchPB <= r.Flow.FreePB+1e-6 {
		return line + " A " + window + "-hour prefetch stages " + staged + " and fits in that free space."
	}
	return line + " A " + window + "-hour prefetch stages " + staged + ", which is larger than the free space."
}

func nvmeLine(r Result) string {
	return "NVMe serves " + fmtBW(r.Flow.NVMeServedGBps) + " of compute reads at a " +
		fmtPct(r.Flow.NVMeHitRate) + " hit rate, leaving " + fmtBW(r.Flow.HDDComputeReadGBps) + " of compute reads on HDD."
}

func minimumLine(r Result) string {
	var b strings.Builder
	b.WriteString("The operational minimum is ")
	if r.Bounds.HardwareMinOK {
		b.WriteString(fmtEB(r.Bounds.HardwareMinEB) + " on the hardware model")
	} else {
		b.WriteString("unreachable on the hardware model")
	}
	if r.Bounds.ObservedEnabled {
		b.WriteString(" and ")
		if r.Bounds.ObservedMinOK {
			b.WriteString(fmtEB(r.Bounds.ObservedMinEB) + " on observed scaling")
		} else {
			b.WriteString("unreachable on observed scaling")
		}
	}
	b.WriteString(". ")
	b.WriteString(listedLine("hardware model", r.Knee.HardwareOK, r.Knee.HardwareTargetEB))
	if r.Bounds.ObservedEnabled {
		b.WriteString(" ")
		b.WriteString(listedLine("observed scaling", r.Knee.ObservedOK, r.Knee.ObservedTargetEB))
	}
	return b.String()
}

func listedLine(name string, ok bool, target float64) string {
	if ok {
		return "The smallest listed tier that stays within budget on " + name + " is " + fmtEB(target) + "."
	}
	return "None of the listed tiers from 0.50 EB to 2.50 EB stay within budget on " + name + "."
}
