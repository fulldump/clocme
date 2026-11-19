package chart

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// compact formats a number with k/M/G suffixes: 950, 1.2k, 34k, 1.5M.
func compact(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	var s string
	switch {
	case v < 1000:
		s = strconv.FormatFloat(math.Round(v), 'f', 0, 64)
	default:
		suffixes := []string{"k", "M", "G", "T"}
		i := 0
		v /= 1000
		for v >= 999.95 && i < len(suffixes)-1 {
			v /= 1000
			i++
		}
		if v < 99.95 {
			s = strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0")
		} else {
			s = strconv.FormatFloat(math.Round(v), 'f', 0, 64)
		}
		s += suffixes[i]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// niceStep rounds a raw step to 1, 2, 2.5 or 5 times a power of ten.
func niceStep(raw float64) float64 {
	if raw <= 0 || math.IsNaN(raw) || math.IsInf(raw, 0) {
		return 1
	}
	exp := math.Floor(math.Log10(raw))
	base := math.Pow(10, exp)
	f := raw / base
	switch {
	case f <= 1:
		f = 1
	case f <= 2:
		f = 2
	case f <= 2.5:
		f = 2.5
	case f <= 5:
		f = 5
	default:
		f = 10
	}
	return f * base
}

// niceTicks returns ticks from 0 to a nice maximum >= maxValue, using about
// `count` intervals. Integer data (minStep=1) never gets fractional ticks.
func niceTicks(maxValue float64, count int, minStep float64) []float64 {
	if count < 1 {
		count = 1
	}
	if maxValue <= 0 {
		maxValue = minStep * float64(count)
		if maxValue <= 0 {
			maxValue = 1
		}
	}
	step := niceStep(maxValue / float64(count))
	if step < minStep {
		step = minStep
	}
	n := int(math.Ceil(maxValue/step - 1e-9))
	ticks := make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		ticks = append(ticks, float64(i)*step)
	}
	return ticks
}

type timeTick struct {
	T     time.Time
	Label string
}

type timeInterval struct {
	years, months, days int
}

var timeIntervals = []timeInterval{
	{0, 0, 1}, {0, 0, 2}, {0, 0, 7}, {0, 0, 14},
	{0, 1, 0}, {0, 2, 0}, {0, 3, 0}, {0, 6, 0},
	{1, 0, 0}, {2, 0, 0}, {5, 0, 0}, {10, 0, 0}, {25, 0, 0}, {50, 0, 0},
}

func (iv timeInterval) approx() time.Duration {
	return time.Duration(iv.years)*365*24*time.Hour +
		time.Duration(iv.months)*30*24*time.Hour +
		time.Duration(iv.days)*24*time.Hour
}

// timeTicks returns aligned, nicely formatted ticks in [from, to] using at
// most maxTicks labels.
func timeTicks(from, to time.Time, maxTicks int) []timeTick {
	from, to = from.UTC(), to.UTC()
	if maxTicks < 2 {
		maxTicks = 2
	}
	span := to.Sub(from)
	iv := timeIntervals[len(timeIntervals)-1]
	for _, candidate := range timeIntervals {
		if span/candidate.approx() <= time.Duration(maxTicks) {
			iv = candidate
			break
		}
	}

	// first aligned tick >= from
	var t time.Time
	switch {
	case iv.years > 0:
		y := from.Year()
		y = int(math.Ceil(float64(y)/float64(iv.years))) * iv.years
		t = time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		for t.Before(from) {
			t = t.AddDate(iv.years, 0, 0)
		}
	case iv.months > 0:
		m := int(from.Month()) - 1
		m = int(math.Ceil(float64(m)/float64(iv.months))) * iv.months
		t = time.Date(from.Year(), time.Month(m+1), 1, 0, 0, 0, 0, time.UTC)
		for t.Before(from) {
			t = t.AddDate(0, iv.months, 0)
		}
	default:
		t = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		if iv.days == 7 || iv.days == 14 {
			// align weeks on mondays
			for t.Weekday() != time.Monday {
				t = t.AddDate(0, 0, 1)
			}
		}
		for t.Before(from) {
			t = t.AddDate(0, 0, iv.days)
		}
	}

	var ticks []timeTick
	for !t.After(to) && len(ticks) < 100 {
		var label string
		switch {
		case iv.years > 0:
			label = t.Format("2006")
		case iv.months > 0:
			label = t.Format("Jan 2006")
			if iv.months < 12 && t.Month() != time.January && len(ticks) > 0 {
				label = t.Format("Jan")
			}
		default:
			label = t.Format("2 Jan")
		}
		ticks = append(ticks, timeTick{T: t, Label: label})
		t = t.AddDate(iv.years, iv.months, iv.days)
	}
	return ticks
}

// fmtCoord rounds to one decimal and strips trailing zeros.
func fmtCoord(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

// FormatCompact formats a count with k/M/G suffixes: 950, 1.2k, 34k, 1.5M.
func FormatCompact(n int64) string {
	return compact(float64(n))
}

// bestTicks tries between lo and hi intervals and picks the tick set whose
// top is closest to maxValue (least wasted vertical space).
func bestTicks(maxValue float64, lo, hi int) []float64 {
	var best []float64
	bestTop := math.Inf(1)
	for c := lo; c <= max(lo, hi); c++ {
		t := niceTicks(maxValue, c, 1)
		top := t[len(t)-1]
		if top < bestTop || (top == bestTop && len(t) > len(best)) {
			best, bestTop = t, top
		}
	}
	return best
}
