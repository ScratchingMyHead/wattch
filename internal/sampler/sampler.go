package sampler

import (
	"sync"
	"time"

	"github.com/ScratchingMyHead/wattch/internal/hwmon"
	"github.com/ScratchingMyHead/wattch/internal/nvml"
	"github.com/ScratchingMyHead/wattch/internal/rapl"
)

type Sample struct {
	Time    time.Time
	Sources map[string]float64 // name -> watts
	Total   float64
}

type Sampler struct {
	mu       sync.Mutex
	rapl     []*rapl.Zone
	hwmon    []*hwmon.Sensor
	nvCount  int
	hasNV    bool
	callback func(Sample)
	lastTime time.Time
}

func New() *Sampler {
	s := &Sampler{}
	s.rapl, _ = rapl.Discover()
	s.hwmon, _ = hwmon.Discover()
	if cnt, err := nvml.Probe(); err == nil && cnt > 0 {
		s.hasNV = true
		s.nvCount = cnt
	}
	s.lastTime = time.Now()
	return s
}

func (s *Sampler) Sources() []string {
	var out []string
	for _, z := range s.rapl {
		out = append(out, z.Name)
	}
	for _, h := range s.hwmon {
		out = append(out, h.Name)
	}
	if s.hasNV {
		for i := 0; i < s.nvCount; i++ {
			out = append(out, "nvidia:"+string(rune('0'+i)))
		}
	}
	return out
}

func (s *Sampler) HasRapl() bool { return len(s.rapl) > 0 }
func (s *Sampler) RaplAccessible() bool { return rapl.IsAccessible() }
func (s *Sampler) Poll() Sample {
	now := time.Now()
	dt := now.Sub(s.lastTime).Seconds()
	if dt <= 0 {
		dt = 1
	}
	s.lastTime = now
	m := make(map[string]float64)
	total := 0.0
	for _, z := range s.rapl {
		w, err := z.ReadWatts(dt)
		if err != nil {
			continue
		}
		// first read returns 0, skip but still record
		m[z.Name] = w
		total += w
	}
	for _, h := range s.hwmon {
		w, err := h.ReadWatts()
		if err != nil {
			continue
		}
		m[h.Name] = w
		total += w
	}
	if s.hasNV {
		for i := 0; i < s.nvCount; i++ {
			w, name, err := nvml.GetPowerUsage(i)
			if err != nil {
				continue
			}
			m[name] = w
			total += w
		}
	}
	return Sample{Time: now, Sources: m, Total: total}
}

func (s *Sampler) Close() {
	if s.hasNV {
		nvml.Shutdown()
	}
}
