package rapl

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Zone struct {
	Name         string
	Path         string
	MaxRange     uint64
	PrevEnergy   uint64
	HasPrev      bool
}

func Discover() ([]*Zone, error) {
	var zones []*Zone
	matches, _ := filepath.Glob("/sys/class/powercap/intel-rapl:*")
	// also /sys/class/powercap/intel-rapl (parent not having energy)
	for _, p := range matches {
		// we want dirs that have energy_uj file
		namePath := filepath.Join(p, "name")
		energyPath := filepath.Join(p, "energy_uj")
		maxPath := filepath.Join(p, "max_energy_range_uj")
		nameB, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameB))
		if _, err := os.Stat(energyPath); err != nil {
			continue
		}
		var max uint64 = 1<<63 - 1
		if b, err := os.ReadFile(maxPath); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
				max = v
			}
		}
		zones = append(zones, &Zone{Name: name, Path: energyPath, MaxRange: max})
	}
	return zones, nil
}

func (z *Zone) ReadWatts(dtSec float64) (float64, error) {
	b, err := os.ReadFile(z.Path)
	if err != nil {
		return 0, err
	}
	cur, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, err
	}
	if !z.HasPrev {
		z.PrevEnergy = cur
		z.HasPrev = true
		return 0, nil
	}
	var delta uint64
	if cur >= z.PrevEnergy {
		delta = cur - z.PrevEnergy
	} else {
		// wrap
		delta = (z.MaxRange - z.PrevEnergy) + cur
	}
	z.PrevEnergy = cur
	if dtSec <= 0 {
		return 0, nil
	}
	microJ := float64(delta)
	watts := microJ / dtSec / 1e6
	return watts, nil
}

func IsAccessible() bool {
	zones, _ := Discover()
	if len(zones) == 0 {
		return false
	}
	// try reading first
	_, err := os.ReadFile(zones[0].Path)
	return err == nil
}
