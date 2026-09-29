package sim

import (
	"math"
	"strconv"
	"strings"
)

func trimFloat(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-" || s == "-0" {
		return "0"
	}
	return s
}

func fmtInt(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}

func fmtEB(v float64) string {
	return trimFloat(v, 2) + " EB"
}

func fmtPB(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	if v >= 1000 {
		return sign + trimFloat(v/1000, 2) + " EB"
	}
	if v >= 10 {
		return sign + trimFloat(v, 1) + " PB"
	}
	return sign + trimFloat(v, 2) + " PB"
}

func fmtBW(gbps float64) string {
	if gbps < 0 {
		return "-" + fmtBW(-gbps)
	}
	switch {
	case gbps >= 1000:
		return trimFloat(gbps/1000, 2) + " TB/s"
	case gbps >= 10:
		return trimFloat(gbps, 1) + " GB/s"
	case gbps >= 1:
		return trimFloat(gbps, 2) + " GB/s"
	default:
		return trimFloat(gbps*1000, 1) + " MB/s"
	}
}

func fmtMBps(mbps float64) string {
	if mbps < 0 {
		return "-" + fmtMBps(-mbps)
	}
	if mbps >= 1000 {
		return trimFloat(mbps/1000, 2) + " GB/s"
	}
	return trimFloat(mbps, 1) + " MB/s"
}

func fmtMB(mb float64) string {
	if mb >= 1 {
		return trimFloat(mb, 2) + " MB"
	}
	return trimFloat(mb*1000, 0) + " KB"
}

func fmtPct(frac float64) string {
	return trimFloat(frac*100, 1) + "%"
}

func fmtNum(v float64) string {
	return trimFloat(v, 2)
}

func fmtCHF(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1e9:
		return sign + "CHF " + trimFloat(v/1e9, 2) + " billion"
	case v >= 1e6:
		return sign + "CHF " + trimFloat(v/1e6, 2) + " million"
	default:
		return sign + "CHF " + fmtInt(int(math.Round(v)))
	}
}

func fmtHours(h float64) string {
	if h < 0 {
		h = 0
	}
	if h >= 48 {
		return trimFloat(h/24, 1) + " days"
	}
	return trimFloat(h, 1) + " hours"
}
