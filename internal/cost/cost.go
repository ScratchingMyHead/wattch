package cost

import (
	"strings"
	"time"

	"github.com/ScratchingMyHead/wattch/internal/config"
)

var dayMap = map[string]time.Weekday{
	"Mon": time.Monday, "Tue": time.Tuesday, "Wed": time.Wednesday,
	"Thu": time.Thursday, "Fri": time.Friday, "Sat": time.Saturday, "Sun": time.Sunday,
	"Monday": time.Monday, "Tuesday": time.Tuesday, "Wednesday": time.Wednesday,
	"Thursday": time.Thursday, "Friday": time.Friday, "Saturday": time.Saturday, "Sunday": time.Sunday,
}

func parseHM(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	// expect HH:MM
	if len(s) < 4 {
		return 0, false
	}
	var h, m int
	// allow 07:00 or 7:00
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	// simple parse
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return 0, false
		}
		h = h*10 + int(c-'0')
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return 0, false
		}
		m = m*10 + int(c-'0')
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func ruleMatches(rule config.TariffRule, t time.Time) bool {
	// days check
	if len(rule.Days) > 0 {
		wd := t.Weekday()
		matched := false
		for _, d := range rule.Days {
			if dw, ok := dayMap[d]; ok && dw == wd {
				matched = true
				break
			}
			// case-insensitive
			if dw, ok := dayMap[strings.Title(strings.ToLower(d))]; ok && dw == wd {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// time check
	if rule.Start == "" && rule.End == "" {
		return true
	}
	startMin, hasStart := parseHM(rule.Start)
	endMin, hasEnd := parseHM(rule.End)
	if !hasStart && !hasEnd {
		return true
	}
	// if only one set, treat missing as 0 or 24h
	if !hasStart {
		startMin = 0
	}
	if !hasEnd {
		endMin = 24 * 60
	}
	nowMin := t.Hour()*60 + t.Minute()
	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	// overnight wrap e.g. 22:00-06:00
	return nowMin >= startMin || nowMin < endMin
}

// PriceAt returns price per kWh and label of matched rule, or default
func PriceAt(tariff config.Tariff, tm time.Time) (float64, string) {
	for _, r := range tariff.Rules {
		if ruleMatches(r, tm) {
			return r.Price, r.Label
		}
	}
	return tariff.DefaultPrice, "default"
}

type Accumulator struct {
	Tariff config.Tariff
	State  config.State
}

func (a *Accumulator) Add(watts float64, dtSec float64, at time.Time) float64 {
	kwh := watts * dtSec / 3600.0 / 1000.0
	price, _ := PriceAt(a.Tariff, at)
	cost := kwh * price
	a.State.AccumKwh += kwh
	a.State.AccumCost += cost
	if a.State.SinceUnix == 0 {
		a.State.SinceUnix = at.Unix()
	}
	return cost
}

func (a *Accumulator) Reset(at time.Time) {
	a.State.AccumKwh = 0
	a.State.AccumCost = 0
	a.State.SinceUnix = at.Unix()
}
