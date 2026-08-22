package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Block is a 30-minute aggregate
type Block struct {
	StartUnix int64             `json:"start_unix"`
	AvgWatts  float64           `json:"avg_watts"`
	Sources   map[string]float64 `json:"sources"` // avg per source
	Kwh       float64           `json:"kwh"`
	Cost      float64           `json:"cost"`
	Count     int               `json:"count"`
}

// Day holds 48 blocks for a calendar day
type Day struct {
	Date   string  `json:"date"` // YYYY-MM-DD
	Blocks []Block `json:"blocks"` // len 48, may have empty slots
}

func dir() string {
	d, _ := os.UserConfigDir()
	if d == "" {
		d = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(d, "wattch", "history")
}

func pathFor(date string) string {
	return filepath.Join(dir(), date+".json")
}

func blockIndex(t time.Time) int {
	return t.Hour()*2 + t.Minute()/30
}

func startOfBlock(t time.Time) time.Time {
	// truncate to 30m
	year, month, day := t.Date()
	hour := t.Hour()
	min := (t.Minute() / 30) * 30
	// use local time
	return time.Date(year, month, day, hour, min, 0, 0, t.Location())
}

type Store struct {
	mu      sync.Mutex
	curDate string
	curDay  *Day
	// accumulator for current block
	blockAgg map[string]float64 // sum watts per source
	blockSum float64
	blockCnt int
	curBlock int
	loc      *time.Location
}

func NewStore() *Store {
	s := &Store{
		blockAgg: make(map[string]float64),
		loc:      time.Local,
	}
	now := time.Now()
	s.curDate = now.Format("2006-01-02")
	s.curBlock = blockIndex(now)
	s.curDay = s.loadOrCreate(s.curDate)
	return s
}

func (s *Store) loadOrCreate(date string) *Day {
	p := pathFor(date)
	b, err := os.ReadFile(p)
	if err != nil {
		// create empty 48 blocks
		blocks := make([]Block, 48)
		base, _ := time.Parse("2006-01-02", date)
		for i := 0; i < 48; i++ {
			start := time.Date(base.Year(), base.Month(), base.Day(), i/2, (i%2)*30, 0, 0, s.loc)
			blocks[i] = Block{
				StartUnix: start.Unix(),
				Sources:   map[string]float64{},
			}
		}
		return &Day{Date: date, Blocks: blocks}
	}
	var d Day
	if err := json.Unmarshal(b, &d); err != nil {
		// fallback empty
		blocks := make([]Block, 48)
		base, _ := time.Parse("2006-01-02", date)
		for i := 0; i < 48; i++ {
			start := time.Date(base.Year(), base.Month(), base.Day(), i/2, (i%2)*30, 0, 0, s.loc)
			blocks[i] = Block{StartUnix: start.Unix(), Sources: map[string]float64{}}
		}
		return &Day{Date: date, Blocks: blocks}
	}
	// ensure 48
	if len(d.Blocks) != 48 {
		// pad
		newBlocks := make([]Block, 48)
		base, _ := time.Parse("2006-01-02", d.Date)
		for i := 0; i < 48; i++ {
			if i < len(d.Blocks) {
				newBlocks[i] = d.Blocks[i]
			} else {
				start := time.Date(base.Year(), base.Month(), base.Day(), i/2, (i%2)*30, 0, 0, s.loc)
				newBlocks[i] = Block{StartUnix: start.Unix(), Sources: map[string]float64{}}
			}
			if newBlocks[i].Sources == nil {
				newBlocks[i].Sources = map[string]float64{}
			}
		}
		d.Blocks = newBlocks
	}
	for i := range d.Blocks {
		if d.Blocks[i].Sources == nil {
			d.Blocks[i].Sources = map[string]float64{}
		}
	}
	return &d
}

func (s *Store) save() error {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.curDay, "", "  ")
	if err != nil {
		return err
	}
	p := pathFor(s.curDate)
	return os.WriteFile(p, b, 0644)
}

// AddSample should be called per poll (e.g. 1s) with total and per-source watts
func (s *Store) AddSample(t time.Time, total float64, sources map[string]float64, costPerKwh float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	date := t.Format("2006-01-02")
	idx := blockIndex(t)

	// if day changed, flush previous day and switch
	if date != s.curDate {
		// finalize current block into curDay before switching
		s.flushBlockLocked()
		_ = s.save()
		s.curDate = date
		s.curDay = s.loadOrCreate(date)
		s.curBlock = idx
		s.blockAgg = make(map[string]float64)
		s.blockSum = 0
		s.blockCnt = 0
	}
	// if block changed, flush previous
	if idx != s.curBlock {
		s.flushBlockLocked()
		s.curBlock = idx
		s.blockAgg = make(map[string]float64)
		s.blockSum = 0
		s.blockCnt = 0
	}
	// accumulate
	s.blockSum += total
	s.blockCnt++
	for k, v := range sources {
		s.blockAgg[k] += v
	}
	// we also periodically flush to file every ~30 samples? but block not yet complete
	// For durability, we update current block's provisional avg so viewer sees live data
	// Compute provisional
	provisionalAvg := s.blockSum / float64(s.blockCnt)
	provisionalSources := make(map[string]float64)
	for k, sum := range s.blockAgg {
		provisionalSources[k] = sum / float64(s.blockCnt)
	}
	// provisional kwh/cost for elapsed portion of block: watts * elapsedSec /3600/1000
	// We estimate based on samples so far: kwh = avg * elapsedSec/3600/1000
	// elapsedSec approximated by blockCnt * sample interval? Instead use actual time within block:
	blockStart := startOfBlock(t)
	elapsedSec := t.Sub(blockStart).Seconds()
	if elapsedSec < 1 {
		elapsedSec = float64(s.blockCnt) // fallback 1s per sample
	}
	kwh := provisionalAvg * elapsedSec / 3600.0 / 1000.0
	cost := kwh * costPerKwh

	b := &s.curDay.Blocks[idx]
	b.AvgWatts = provisionalAvg
	b.Sources = provisionalSources
	b.Count = s.blockCnt
	b.Kwh = kwh
	b.Cost = cost
	// flush to disk every 30 seconds (approx each 30 samples if 1s)
	if s.blockCnt%30 == 0 {
		_ = s.save()
	}
}

func (s *Store) flushBlockLocked() {
	// called when block completes; ensure saved
	_ = s.save()
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) FlushCurrent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.save()
}

// LoadDay loads a specific date's day (or empty if not exists)
func LoadDay(date string) (*Day, error) {
	p := pathFor(date)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// return empty
			blocks := make([]Block, 48)
			base, _ := time.Parse("2006-01-02", date)
			loc := time.Local
			for i := 0; i < 48; i++ {
				start := time.Date(base.Year(), base.Month(), base.Day(), i/2, (i%2)*30, 0, 0, loc)
				blocks[i] = Block{StartUnix: start.Unix(), Sources: map[string]float64{}}
			}
			return &Day{Date: date, Blocks: blocks}, nil
		}
		return nil, err
	}
	var d Day
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	for i := range d.Blocks {
		if d.Blocks[i].Sources == nil {
			d.Blocks[i].Sources = map[string]float64{}
		}
	}
	return &d, nil
}

// ListDates returns sorted dates available
func ListDates() ([]string, error) {
	entries, err := os.ReadDir(dir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) == 15 && name[10:] == ".json" { // YYYY-MM-DD.json = 15
			out = append(out, name[:10])
		}
	}
	return out, nil
}
