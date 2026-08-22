package hwmon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Sensor struct {
	Name  string // e.g. amdgpu PPT
	Path  string // power1_input
	Label string
}

func Discover() ([]*Sensor, error) {
	var sensors []*Sensor
	hws, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, d := range hws {
		nameB, err := os.ReadFile(filepath.Join(d, "name"))
		if err != nil {
			continue
		}
		hwName := strings.TrimSpace(string(nameB))
		// find power*_input files
		matches, _ := filepath.Glob(filepath.Join(d, "power*_input"))
		for _, p := range matches {
			base := filepath.Base(p) // power1_input
			prefix := strings.TrimSuffix(base, "_input")
			labelPath := filepath.Join(d, prefix+"_label")
			label := ""
			if b, err := os.ReadFile(labelPath); err == nil {
				label = strings.TrimSpace(string(b))
			}
			display := hwName
			if label != "" {
				display = hwName + ":" + label
			} else {
				// use power1 etc
				display = hwName + ":" + prefix
			}
			sensors = append(sensors, &Sensor{Name: display, Path: p, Label: label})
		}
	}
	return sensors, nil
}

func (s *Sensor) ReadWatts() (float64, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, err
	}
	// value is microwatts
	return float64(v) / 1e6, nil
}
