package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Geometry struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Config struct {
	Currency      string            `json:"currency"`
	DefaultPrice  float64           `json:"default_price"`
	SampleMs      int               `json:"sample_ms"`
	HistoryS      int               `json:"history_s"`
	Frameless     bool              `json:"frameless"`
	AlwaysOnTop   bool              `json:"always_on_top"`
	ShowLegend    bool              `json:"show_legend"`
	Geometry      Geometry          `json:"geometry"`
	Colors        map[string]string `json:"colors"`
}

type TariffRule struct {
	Label string   `json:"label"`
	Days  []string `json:"days"` // Mon Tue Wed Thu Fri Sat Sun ; empty = all
	Start string   `json:"start,omitempty"` // HH:MM
	End   string   `json:"end,omitempty"`
	Price float64  `json:"price"`
}

type Tariff struct {
	Currency     string       `json:"currency"`
	DefaultPrice float64      `json:"default_price"`
	Rules        []TariffRule `json:"rules"`
}

type State struct {
	AccumKwh  float64 `json:"accum_kwh"`
	AccumCost float64 `json:"accum_cost"`
	SinceUnix int64   `json:"since_unix"`
}

var mu sync.Mutex

func dir() string {
	d, _ := os.UserConfigDir()
	if d == "" {
		d = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(d, "wattch")
}

func containsKey(data []byte, key string) bool {
	// crude but sufficient: look for "key" quoted
	needle := `"` + key + `"`
	// use simple search
	for i := 0; i+len(needle) <= len(data); i++ {
		if string(data[i:i+len(needle)]) == needle {
			return true
		}
	}
	return false
}

func ensureDir() error {
	return os.MkdirAll(dir(), 0755)
}

func DefaultConfig() Config {
	return Config{
		Currency:     "$",
		DefaultPrice: 0.25,
		SampleMs:     1000,
		HistoryS:     300,
		Frameless:    false,
		AlwaysOnTop:  false,
		ShowLegend:   true,
		Geometry:     Geometry{X: 100, Y: 100, W: 360, H: 200},
		Colors:       map[string]string{},
	}
}

func DefaultTariff() Tariff {
	return Tariff{
		Currency:     "$",
		DefaultPrice: 0.25,
		Rules:        []TariffRule{},
	}
}

func LoadConfig() (Config, error) {
	mu.Lock()
	defer mu.Unlock()
	var c Config
	p := filepath.Join(dir(), "config.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return DefaultConfig(), err
	}
	// migrate_show_legend: default true if key missing
	if !containsKey(b, "show_legend") {
		c.ShowLegend = true
	}
	if c.Currency == "" {
		c.Currency = "$"
	}
	if c.SampleMs <= 0 {
		c.SampleMs = 1000
	}
	if c.HistoryS <= 0 {
		c.HistoryS = 300
	}
	if c.Colors == nil {
		c.Colors = map[string]string{}
	}
	return c, nil
}

func SaveConfig(c Config) error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	p := filepath.Join(dir(), "config.json")
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

func LoadTariff() (Tariff, error) {
	mu.Lock()
	defer mu.Unlock()
	var t Tariff
	p := filepath.Join(dir(), "tariff.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// try legacy config for migration
			cfg, _ := func() (Config, error) {
				// avoid deadlock - read directly
				b2, e2 := os.ReadFile(filepath.Join(dir(), "config.json"))
				if e2 != nil {
					return DefaultConfig(), e2
				}
				var c Config
				json.Unmarshal(b2, &c)
				return c, nil
			}()
			_ = cfg
			return DefaultTariff(), nil
		}
		return t, err
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return DefaultTariff(), err
	}
	if t.Currency == "" {
		t.Currency = "$"
	}
	if t.Rules == nil {
		t.Rules = []TariffRule{}
	}
	return t, nil
}

func SaveTariff(t Tariff) error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	p := filepath.Join(dir(), "tariff.json")
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

func LoadState() (State, error) {
	mu.Lock()
	defer mu.Unlock()
	var s State
	p := filepath.Join(dir(), "state.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return s, err
	}
	json.Unmarshal(b, &s)
	return s, nil
}

func SaveState(s State) error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	p := filepath.Join(dir(), "state.json")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}
