package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
	"github.com/ScratchingMyHead/wattch/internal/history"
)

type HistoryViewer struct {
	Window    *gtk.Window
	Area      *gtk.DrawingArea
	DateLabel *gtk.Label
	Current   string // YYYY-MM-DD
}

// ShowHistory opens a larger window with 30-min blocks for daily view
func (a *App) ShowHistory() {
	if a.HistoryViewer != nil && a.HistoryViewer.Window != nil {
		a.HistoryViewer.Window.Present()
		return
	}
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle("wattch — History (30-min blocks)")
	win.SetDefaultSize(800, 500)
	win.SetPosition(gtk.WIN_POS_CENTER)

	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	vbox.SetMarginTop(8)
	vbox.SetMarginStart(8)
	vbox.SetMarginEnd(8)
	vbox.SetMarginBottom(8)
	win.Add(vbox)

	// controls
	ctrlBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	prevBtn, _ := gtk.ButtonNewWithLabel("◀ Prev")
	nextBtn, _ := gtk.ButtonNewWithLabel("Next ▶")
	todayBtn, _ := gtk.ButtonNewWithLabel("Today")
	dateLbl, _ := gtk.LabelNew("")
	dateLbl.SetHExpand(true)
	ctrlBox.PackStart(prevBtn, false, false, 0)
	ctrlBox.PackStart(dateLbl, true, true, 0)
	ctrlBox.PackStart(todayBtn, false, false, 0)
	ctrlBox.PackStart(nextBtn, false, false, 0)
	vbox.PackStart(ctrlBox, false, false, 0)

	// summary label
	summaryLbl, _ := gtk.LabelNew("")
	summaryLbl.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(summaryLbl, false, false, 0)

	area, _ := gtk.DrawingAreaNew()
	area.SetHExpand(true)
	area.SetVExpand(true)
	vbox.PackStart(area, true, true, 0)

	// legend
	legendBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	vbox.PackStart(legendBox, false, false, 0)

	hv := &HistoryViewer{Window: win, Area: area, DateLabel: dateLbl, Current: time.Now().Format("2006-01-02")}
	a.HistoryViewer = hv

	var dayData *history.Day

	update := func() {
		d, err := history.LoadDay(hv.Current)
		if err != nil {
			summaryLbl.SetText(fmt.Sprintf("Error loading %s: %v", hv.Current, err))
			return
		}
		dayData = d
		dateLbl.SetText(hv.Current)
		// summary
		totalKwh := 0.0
		totalCost := 0.0
		peakW := 0.0
		for _, b := range d.Blocks {
			totalKwh += b.Kwh
			totalCost += b.Cost
			if b.AvgWatts > peakW {
				peakW = b.AvgWatts
			}
		}
		summaryLbl.SetText(fmt.Sprintf("Total: %.2f kWh  •  Cost %s%.2f  •  Peak %.0f W", totalKwh, a.Tariff.Currency, totalCost, peakW))

		// legend
		// collect sources present
		srcSet := map[string]bool{}
		for _, b := range d.Blocks {
			for k := range b.Sources {
				srcSet[k] = true
			}
		}
		var srcs []string
		for k := range srcSet {
			srcs = append(srcs, k)
		}
		sort.Strings(srcs)
		legendBox.GetChildren().Foreach(func(item interface{}) {
			if w, ok := item.(*gtk.Widget); ok {
				legendBox.Remove(w)
			}
		})
		for i, src := range srcs {
			lbl, _ := gtk.LabelNew("")
			c := palette[i%len(palette)]
			if col, ok := a.Colors[src]; ok && col != "" {
				c = col
			}
			lbl.SetMarkup(fmt.Sprintf(`<span foreground="%s">●</span> %s`, c, src))
			legendBox.PackStart(lbl, false, false, 0)
		}
		tLbl, _ := gtk.LabelNew("")
		tLbl.SetMarkup(`<span foreground="white">●</span> total`)
		legendBox.PackStart(tLbl, false, false, 0)
		legendBox.ShowAll()
		area.QueueDraw()
	}

	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) bool {
		w := area.GetAllocatedWidth()
		h := area.GetAllocatedHeight()
		if dayData == nil {
			return false
		}
		drawDailyBlocks(cr, w, h, dayData, a.Colors)
		return false
	})

	prevBtn.Connect("clicked", func() {
		t, _ := time.Parse("2006-01-02", hv.Current)
		hv.Current = t.AddDate(0, 0, -1).Format("2006-01-02")
		update()
	})
	nextBtn.Connect("clicked", func() {
		t, _ := time.Parse("2006-01-02", hv.Current)
		hv.Current = t.AddDate(0, 0, 1).Format("2006-01-02")
		update()
	})
	todayBtn.Connect("clicked", func() {
		hv.Current = time.Now().Format("2006-01-02")
		update()
	})

	win.Connect("destroy", func() {
		a.HistoryViewer = nil
	})

	update()
	win.ShowAll()
}

func drawDailyBlocks(cr *cairo.Context, w, h int, day *history.Day, colors map[string]string) {
	// background
	cr.SetSourceRGB(0.12, 0.12, 0.12)
	cr.Rectangle(0, 0, float64(w), float64(h))
	cr.Fill()

	// find max
	maxW := 50.0
	for _, b := range day.Blocks {
		if b.AvgWatts > maxW {
			maxW = b.AvgWatts
		}
	}
	maxW *= 1.15
	if maxW < 50 {
		maxW = 50
	}
	// grid
	cr.SetSourceRGB(0.25, 0.25, 0.25)
	cr.SetLineWidth(0.5)
	for i := 1; i < 4; i++ {
		y := float64(h) * float64(i) / 4.0
		cr.MoveTo(40, y)
		cr.LineTo(float64(w), y)
		cr.Stroke()
	}
	// Y labels
	cr.SetSourceRGB(0.8, 0.8, 0.8)
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(9)
	cr.MoveTo(4, 12)
	cr.ShowText(fmt.Sprintf("%.0fW", maxW))
	cr.MoveTo(4, float64(h)/2)
	cr.ShowText(fmt.Sprintf("%.0fW", maxW/2))
	cr.MoveTo(4, float64(h)-4)
	cr.ShowText("0W")

	plotW := float64(w - 50)
	plotH := float64(h - 20)
	barW := plotW / 48.0
	// draw per-block bars/lines
	// we draw stacked? Instead draw total as white line and sources as colored lines for clarity
	// For bar view, we draw bars for total, and small dots for sources
	// Let's draw total as bars
	for i, b := range day.Blocks {
		x := 40 + float64(i)*barW
		barH := (b.AvgWatts / maxW) * plotH
		y := float64(h) - 10 - barH
		if b.Count == 0 {
			// empty block - no bar, just gap
			continue
		}
		// total bar in white semi
		cr.SetSourceRGBA(1, 1, 1, 0.9)
		cr.Rectangle(x+1, y, barW-2, barH)
		cr.Fill()
		// source overlays as thin lines on top? draw small colored segments inside bar proportionally?
		// Instead draw stacked: sort sources and stack
		stackY := y + barH
		// compute total of sources for stacking check: should approx equal AvgWatts but total is sum of all watts (including multiple sources, total is sum, so avg per source sum = total) -> we can stack per-source contribution
		// draw stacked colored rectangles from bottom up
		sorted := make([]string, 0, len(b.Sources))
		for k := range b.Sources {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for idx, src := range sorted {
			val := b.Sources[src]
			segH := (val / maxW) * plotH
			stackY -= segH
			var r, g, bl float64
			if hex, ok := colors[src]; ok && hex != "" {
				r, g, bl = hexToRGB(hex)
			} else {
				r, g, bl = colorFor(src, idx)
			}
			cr.SetSourceRGB(r, g, bl)
			cr.Rectangle(x+1, stackY, barW-2, segH)
			cr.Fill()
		}
		// overlay cost as text on top if bar tall enough? skip
	}
	// X labels: time
	cr.SetSourceRGB(0.7, 0.7, 0.7)
	cr.SetFontSize(8)
	for i := 0; i < 48; i += 4 { // every 2 hours
		x := 40 + float64(i)*barW
		hour := i / 2
		label := fmt.Sprintf("%02d:00", hour)
		cr.MoveTo(x, float64(h)-2)
		cr.ShowText(label)
	}
	// outline
	cr.SetSourceRGB(0.4, 0.4, 0.4)
	cr.Rectangle(40, 10, plotW, plotH)
	cr.Stroke()
}
