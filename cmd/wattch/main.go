package main

import (
	"log"
	"os"

	"github.com/gotk3/gotk3/gtk"
	"github.com/ScratchingMyHead/wattch/internal/config"
	"github.com/ScratchingMyHead/wattch/internal/ui"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("load config: %v, using defaults", err)
		cfg = config.DefaultConfig()
	}
	tariff, err := config.LoadTariff()
	if err != nil {
		log.Printf("load tariff: %v", err)
		tariff = config.DefaultTariff()
	}
	// migrate currency/default from config if tariff empty and config has values
	if tariff.Currency == "$" && cfg.Currency != "" && cfg.Currency != "$" {
		tariff.Currency = cfg.Currency
	}
	if tariff.DefaultPrice == 0.25 && cfg.DefaultPrice != 0.25 && cfg.DefaultPrice != 0 {
		tariff.DefaultPrice = cfg.DefaultPrice
	}
	// ensure tariff file exists
	_ = config.SaveTariff(tariff)
	state, _ := config.LoadState()

	gtk.Init(&os.Args)

	app, err := ui.NewApp(cfg, tariff, state)
	if err != nil {
		log.Fatalf("new app: %v", err)
	}

	if err := app.Build(); err != nil {
		log.Fatalf("build: %v", err)
	}

	// handle SIG? just gtk main
	gtk.Main()
	app.Close()
}
