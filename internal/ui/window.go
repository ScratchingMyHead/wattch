package ui

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/ScratchingMyHead/wattch/internal/config"
	"github.com/ScratchingMyHead/wattch/internal/cost"
	"github.com/ScratchingMyHead/wattch/internal/history"
	"github.com/ScratchingMyHead/wattch/internal/sampler"
)

type App struct {
	Window         *gtk.Window
	HeaderLabel    *gtk.Label
	CostLabel      *gtk.Label
	GraphArea      *gtk.DrawingArea
	SettingsBtn    *gtk.Button
	HistoryBtn     *gtk.Button
	LegendBox      *gtk.Box
	Sampler        *sampler.Sampler
	History        *History
	HistoryStore   *history.Store
	HistoryViewer  *HistoryViewer
	Config         config.Config
	Tariff         config.Tariff
	Accum          cost.Accumulator
	Ticker         *time.Ticker
	OrderedSrc     []string
	Colors         map[string]string
	dragActive     bool
	dragOffX       int
	dragOffY       int
}

func NewApp(cfg config.Config, tariff config.Tariff, state config.State) (*App, error) {
	samp := sampler.New()
	maxPoints := cfg.HistoryS * 1000 / cfg.SampleMs
	if maxPoints < 10 {
		maxPoints = 10
	}
	hist := NewHistory(maxPoints)
	acc := cost.Accumulator{Tariff: tariff, State: state}
	if acc.State.SinceUnix == 0 {
		acc.State.SinceUnix = time.Now().Unix()
	}
	hstore := history.NewStore()
	app := &App{
		Sampler:      samp,
		History:      hist,
		HistoryStore: hstore,
		Config:       cfg,
		Tariff:       tariff,
		Accum:        acc,
		Colors:       cfg.Colors,
	}
	if app.Colors == nil {
		app.Colors = map[string]string{}
	}
	return app, nil
}

func (a *App) Build() error {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	a.Window = win
	win.SetTitle("wattch")
	win.SetDefaultSize(a.Config.Geometry.W, a.Config.Geometry.H)
	if a.Config.Geometry.X != 0 || a.Config.Geometry.Y != 0 {
		win.Move(a.Config.Geometry.X, a.Config.Geometry.Y)
	}
	win.SetDecorated(!a.Config.Frameless)
	win.SetKeepAbove(a.Config.AlwaysOnTop)
	win.SetResizable(true)
	// icon — try file paths, fall back to theme name "wattch"
	iconPaths := []string{
		"assets/wattch.svg",
		"etc/icons/hicolor/48x48/apps/wattch.png",
		"/home/rj/src/wattch/assets/wattch.svg",
		"/home/rj/src/wattch/etc/icons/hicolor/48x48/apps/wattch.png",
	}
	iconSet := false
	for _, p := range iconPaths {
		if err := win.SetIconFromFile(p); err == nil {
			iconSet = true
			break
		}
	}
	if !iconSet {
		win.SetIconName("wattch")
	}

	// main vbox
	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	win.Add(vbox)

	// header — wrapped in EventBox so it can be dragged when frameless
	headerEventBox, _ := gtk.EventBoxNew()
	vbox.PackStart(headerEventBox, false, false, 0)
	hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	hbox.SetMarginTop(6)
	hbox.SetMarginBottom(2)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)
	headerEventBox.Add(hbox)

	header, _ := gtk.LabelNew("— W")
	header.SetHAlign(gtk.ALIGN_START)
	header.SetMarkup(`<b>— W</b>`)
	a.HeaderLabel = header
	hbox.PackStart(header, true, true, 0)

	costLbl, _ := gtk.LabelNew("")
	costLbl.SetHAlign(gtk.ALIGN_END)
	a.CostLabel = costLbl
	hbox.PackStart(costLbl, false, false, 0)

	histBtn, _ := gtk.ButtonNewWithLabel("◨")
	histBtn.SetTooltipText("History (30-min daily blocks)")
	a.HistoryBtn = histBtn
	hbox.PackStart(histBtn, false, false, 0)
	histBtn.Connect("clicked", func() {
		a.ShowHistory()
	})

	setBtn, _ := gtk.ButtonNewWithLabel("⚙")
	setBtn.SetTooltipText("Settings")
	a.SettingsBtn = setBtn
	hbox.PackStart(setBtn, false, false, 0)
	setBtn.Connect("clicked", func() {
		a.ShowSettings()
	})

	// drag handling: allow moving window by dragging the header bar when frameless
	// Manual fallback only (WM BeginMoveDrag caused sticky when combined; pure manual is reliable)
	headerEventBox.AddEvents(int(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK | gdk.POINTER_MOTION_MASK | gdk.POINTER_MOTION_HINT_MASK))
	headerEventBox.Connect("button-press-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		if !a.Config.Frameless {
			return false
		}
		btnEv := gdk.EventButtonNewFromEvent(ev)
		if btnEv.Button() != gdk.BUTTON_PRIMARY {
			return false
		}
		wx, wy := win.GetPosition()
		a.dragActive = true
		a.dragOffX = int(btnEv.XRoot()) - wx
		a.dragOffY = int(btnEv.YRoot()) - wy
		return true
	})
	headerEventBox.Connect("button-release-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		btnEv := gdk.EventButtonNewFromEvent(ev)
		if btnEv.Button() == gdk.BUTTON_PRIMARY {
			a.dragActive = false
		}
		return false
	})
	headerEventBox.Connect("motion-notify-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		if !a.dragActive || !a.Config.Frameless {
			return false
		}
		motEv := gdk.EventMotionNewFromEvent(ev)
		xRoot, yRoot := motEv.MotionValRoot()
		nx := int(xRoot) - a.dragOffX
		ny := int(yRoot) - a.dragOffY
		win.Move(nx, ny)
		return false
	})

	// graph
	area, _ := gtk.DrawingAreaNew()
	area.SetHExpand(true)
	area.SetVExpand(true)
	area.SetSizeRequest(200, 100)
	a.GraphArea = area
	vbox.PackStart(area, true, true, 0)

	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) bool {
		w := area.GetAllocatedWidth()
		h := area.GetAllocatedHeight()
		DrawHistory(cr, w, h, a.History, a.Colors, a.OrderedSrc)
		return false
	})

	// legend box below graph
	legendBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	legendBox.SetMarginStart(6)
	legendBox.SetMarginEnd(6)
	legendBox.SetMarginBottom(4)
	vbox.PackStart(legendBox, false, false, 0)
	a.LegendBox = legendBox
	if !a.Config.ShowLegend {
		legendBox.Hide()
	}
	// we will populate dynamically in updateHeader

	win.Connect("configure-event", func() bool {
		// save geometry on move/resize
		x, y := win.GetPosition()
		w, h := win.GetSize()
		a.Config.Geometry = config.Geometry{X: x, Y: y, W: w, H: h}
		// debounce save? immediate
		_ = config.SaveConfig(a.Config)
		return false
	})

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// ticker
	interval := time.Duration(a.Config.SampleMs) * time.Millisecond
	a.Ticker = time.NewTicker(interval)
	go func() {
		for range a.Ticker.C {
			sample := a.Sampler.Poll()
			// cost
			price, _ := cost.PriceAt(a.Tariff, sample.Time)
			// use Accum.Add
			a.Accum.Add(sample.Total, float64(a.Config.SampleMs)/1000.0, sample.Time)
			// daily 30-min block history
			if a.HistoryStore != nil {
				a.HistoryStore.AddSample(sample.Time, sample.Total, sample.Sources, price)
			}
			// save state every 10s? we do every tick for simplicity but throttle
			// update ordered sources
			if len(a.OrderedSrc) == 0 {
				for k := range sample.Sources {
					a.OrderedSrc = append(a.OrderedSrc, k)
				}
				sort.Strings(a.OrderedSrc)
				// ensure colors assigned
				for i, src := range a.OrderedSrc {
					if _, ok := a.Colors[src]; !ok {
						a.Colors[src] = palette[i%len(palette)]
					}
				}
			} else {
				// add new sources if appeared
				exist := map[string]bool{}
				for _, s := range a.OrderedSrc {
					exist[s] = true
				}
				for k := range sample.Sources {
					if !exist[k] {
						a.OrderedSrc = append(a.OrderedSrc, k)
						sort.Strings(a.OrderedSrc)
					}
				}
			}
			a.History.Add(sample)
			// periodically save state
			if time.Now().Unix()%10 == 0 {
				_ = config.SaveState(a.Accum.State)
			}
			glib.IdleAdd(func() {
				a.updateHeader(sample)
				a.GraphArea.QueueDraw()
				// update legend (only if visible)
				if a.Config.ShowLegend {
					legendBox.GetChildren().Foreach(func(item interface{}) {
						if w, ok := item.(*gtk.Widget); ok {
							legendBox.Remove(w)
						}
					})
					for i, src := range a.OrderedSrc {
						lbl, _ := gtk.LabelNew(src)
						c := palette[i%len(palette)]
						if col, ok := a.Colors[src]; ok {
							c = col
						}
						lbl.SetMarkup(fmt.Sprintf(`<span foreground="%s">●</span> %s`, c, src))
						lbl.SetMarginEnd(8)
						legendBox.PackStart(lbl, false, false, 0)
					}
					// total white
					tLbl, _ := gtk.LabelNew("")
					tLbl.SetMarkup(`<span foreground="white">●</span> total`)
					legendBox.PackStart(tLbl, false, false, 0)
					legendBox.ShowAll()
				}
			})
		}
	}()

	win.ShowAll()
	return nil
}

func (a *App) updateHeader(s sampler.Sample) {
	totalStr := fmt.Sprintf("%.1f W", s.Total)
	if s.Total >= 1000 {
		totalStr = fmt.Sprintf("%.2f kW", s.Total/1000)
	}
	a.HeaderLabel.SetMarkup(fmt.Sprintf(`<b>%s</b>`, totalStr))
	since := time.Unix(a.Accum.State.SinceUnix, 0).Format("2006-01-02 15:04")
	costStr := fmt.Sprintf("%s%.2f since %s", a.Tariff.Currency, a.Accum.State.AccumCost, since)
	a.CostLabel.SetText(costStr)
	// update window title for taskbar?
	a.Window.SetTitle(fmt.Sprintf("wattch — %.1f W", s.Total))
}

func (a *App) SetShowLegend(show bool) {
	a.Config.ShowLegend = show
	_ = config.SaveConfig(a.Config)
	if a.LegendBox != nil {
		if show {
			a.LegendBox.Show()
			// trigger immediate redraw of legend on next sample; also force current redraw
			a.LegendBox.ShowAll()
		} else {
			a.LegendBox.Hide()
		}
	}
}

func (a *App) Close() {
	if a.Ticker != nil {
		a.Ticker.Stop()
	}
	_ = config.SaveState(a.Accum.State)
	_ = config.SaveConfig(a.Config)
	if a.HistoryStore != nil {
		_ = a.HistoryStore.Save()
	}
	if a.Sampler != nil {
		a.Sampler.Close()
	}
	log.Println("wattch closed")
}
