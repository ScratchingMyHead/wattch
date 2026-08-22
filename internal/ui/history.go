package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/ScratchingMyHead/wattch/internal/history"
)

type HistoryViewer struct {
	Window    *gtk.Window
	Area      *gtk.DrawingArea
	DateLabel *gtk.Label
	HoverLabel *gtk.Label
	Current   string // YYYY-MM-DD
	dayData   *history.Day
	currency  string
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

	// hover detail label (below graph, above legend)
	hoverLbl, _ := gtk.LabelNew("")
	hoverLbl.SetHAlign(gtk.ALIGN_START)
	hoverLbl.SetMarkup(`<i>Hover over a bar for details</i>`)
	vbox.PackStart(hoverLbl, false, false, 0)

	// legend
	legendBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	vbox.PackStart(legendBox, false, false, 0)

	hv := &HistoryViewer{Window: win, Area: area, DateLabel: dateLbl, HoverLabel: hoverLbl, Current: time.Now().Format("2006-01-02"), currency: a.Tariff.Currency}
	a.HistoryViewer = hv

	update := func() {
		d, err := history.LoadDay(hv.Current)
		if err != nil {
			summaryLbl.SetText(fmt.Sprintf("Error loading %s: %v", hv.Current, err))
			return
		}
		hv.dayData = d
		hv.currency = a.Tariff.Currency
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
		hoverLbl.SetMarkup(`<i>Hover over a bar for details</i>`)

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
		// cost legend (y2) — purple to avoid yellow bar collision
		costLbl, _ := gtk.LabelNew("")
		costLbl.SetMarkup(`<span foreground="#8e44ad">●</span> cost`)
		legendBox.PackStart(costLbl, false, false, 0)
		legendBox.ShowAll()
		area.QueueDraw()
	}

	// hover handling
	area.AddEvents(int(gdk.POINTER_MOTION_MASK | gdk.LEAVE_NOTIFY_MASK))
	area.Connect("motion-notify-event", func(_ *gtk.DrawingArea, ev *gdk.Event) bool {
		if hv.dayData == nil {
			return false
		}
		mot := gdk.EventMotionNewFromEvent(ev)
		x, _ := mot.MotionVal()
		w := area.GetAllocatedWidth()
		plotW := float64(w - 80) // 40 left + 30 right + margins
		barW := plotW / 48.0
		idx := int((x - 40) / barW)
		if idx < 0 || idx >= 48 {
			hoverLbl.SetMarkup(`<i>Hover over a bar for details</i>`)
			area.SetTooltipText("")
			return false
		}
		b := hv.dayData.Blocks[idx]
		if b.Count == 0 {
			hoverLbl.SetMarkup(fmt.Sprintf(`<b>%02d:%02d–%02d:%02d</b> — no data`, idx/2, (idx%2)*30, (idx/2)+((idx%2)*30+30)/60, ((idx%2)*30+30)%60))
			area.SetTooltipText("No data for this half-hour")
			return false
		}
		start := time.Unix(b.StartUnix, 0)
		end := start.Add(30 * time.Minute)
		// build per-source breakdown sorted
		type kv struct {
			k string
			v float64
		}
		var kvs []kv
		for k, v := range b.Sources {
			kvs = append(kvs, kv{k, v})
		}
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].v > kvs[j].v })
		srcParts := ""
		for i, kv := range kvs {
			if i > 0 {
				srcParts += ", "
			}
			srcParts += fmt.Sprintf("%s %.0fW", kv.k, kv.v)
		}
		if srcParts == "" {
			srcParts = "—"
		}
		hoverText := fmt.Sprintf(`<b>%s – %s</b>  %.0fW avg  •  %.3f kWh  •  %s%.4f`,
			start.Format("15:04"), end.Format("15:04"), b.AvgWatts, b.Kwh, hv.currency, b.Cost)
		hoverLbl.SetMarkup(hoverText + fmt.Sprintf(`  <span foreground="#888">[%s]</span>`, srcParts))
		tooltip := fmt.Sprintf("%s – %s\n%.0f W avg\n%.3f kWh\n%s%.4f\n%s",
			start.Format("15:04"), end.Format("15:04"), b.AvgWatts, b.Kwh, hv.currency, b.Cost, srcParts)
		area.SetTooltipText(tooltip)
		return false
	})
	area.Connect("leave-notify-event", func() bool {
		hoverLbl.SetMarkup(`<i>Hover over a bar for details</i>`)
		return false
	})

	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) bool {
		w := area.GetAllocatedWidth()
		h := area.GetAllocatedHeight()
		if hv.dayData == nil {
			return false
		}
		drawDailyBlocks(cr, w, h, hv.dayData, a.Colors, hv.currency)
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

func drawDailyBlocks(cr *cairo.Context, w, h int, day *history.Day, colors map[string]string, currency string) {
	// background
	cr.SetSourceRGB(0.12, 0.12, 0.12)
	cr.Rectangle(0, 0, float64(w), float64(h))
	cr.Fill()

	// find max for watts (left) and cost (right y2)
	maxW := 50.0
	maxCost := 0.01
	for _, b := range day.Blocks {
		if b.AvgWatts > maxW {
			maxW = b.AvgWatts
		}
		if b.Cost > maxCost {
			maxCost = b.Cost
		}
	}
	maxW *= 1.15
	if maxW < 50 {
		maxW = 50
	}
	maxCost *= 1.15
	if maxCost < 0.01 {
		maxCost = 0.01
	}
	// grid
	cr.SetSourceRGB(0.25, 0.25, 0.25)
	cr.SetLineWidth(0.5)
	for i := 1; i < 4; i++ {
		y := float64(h) * float64(i) / 4.0
		cr.MoveTo(40, y)
		cr.LineTo(float64(w)-35, y)
		cr.Stroke()
	}
	// Y labels left (watts)
	cr.SetSourceRGB(0.8, 0.8, 0.8)
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(9)
	cr.MoveTo(4, 12)
	cr.ShowText(fmt.Sprintf("%.0fW", maxW))
	cr.MoveTo(4, float64(h)/2)
	cr.ShowText(fmt.Sprintf("%.0fW", maxW/2))
	cr.MoveTo(4, float64(h)-4)
	cr.ShowText("0W")
	// Y labels right (cost) — y2 purple
	cr.SetSourceRGB(0.56, 0.27, 0.68) // #8e44ad purple
	cr.MoveTo(float64(w)-30, 12)
	cr.ShowText(fmt.Sprintf("%s%.2f", currency, maxCost))
	cr.MoveTo(float64(w)-30, float64(h)/2)
	cr.ShowText(fmt.Sprintf("%s%.2f", currency, maxCost/2))
	cr.MoveTo(float64(w)-30, float64(h)-4)
	cr.ShowText(fmt.Sprintf("%s0", currency))

	plotW := float64(w - 80) // 40 left + 35 right + gaps
	plotH := float64(h - 20)
	barW := plotW / 48.0
	// draw per-block stacked watts bars
	for i, b := range day.Blocks {
		x := 40 + float64(i)*barW
		barH := (b.AvgWatts / maxW) * plotH
		y := float64(h) - 10 - barH
		if b.Count == 0 {
			continue
		}
		stackY := y + barH
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
	}
	// draw cost line on y2 (right axis) — purple line with dots
	cr.SetSourceRGB(0.56, 0.27, 0.68) // #8e44ad
	cr.SetLineWidth(1.8)
	first := true
	for i, b := range day.Blocks {
		if b.Count == 0 {
			first = true
			continue
		}
		x := 40 + float64(i)*barW + barW/2
		y := float64(h) - 10 - (b.Cost/maxCost)*plotH
		if first {
			cr.MoveTo(x, y)
			first = false
		} else {
			cr.LineTo(x, y)
		}
	}
	cr.Stroke()
	// cost dots
	for i, b := range day.Blocks {
		if b.Count == 0 {
			continue
		}
		x := 40 + float64(i)*barW + barW/2
		y := float64(h) - 10 - (b.Cost/maxCost)*plotH
		cr.Arc(x, y, 2.2, 0, 2*3.14159)
		cr.Fill()
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
	// right y2 outline hint (thin purple)
	cr.SetSourceRGB(0.56, 0.27, 0.68)
	cr.SetLineWidth(0.6)
	cr.Rectangle(float64(w)-35, 10, 0, plotH)
	cr.Stroke()
}
