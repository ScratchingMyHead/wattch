package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/ScratchingMyHead/wattch/internal/config"
	"github.com/ScratchingMyHead/wattch/internal/sampler"
)

func (a *App) ShowSettings() {
	dialog, _ := gtk.DialogNew()
	dialog.SetTitle("wattch Settings")
	dialog.SetTransientFor(a.Window)
	dialog.SetModal(true)
	dialog.SetDefaultSize(520, 480)
	dialog.AddButton("Close", gtk.RESPONSE_CLOSE)

	content, _ := dialog.GetContentArea()
	notebook, _ := gtk.NotebookNew()
	content.PackStart(notebook, true, true, 6)

	// General tab
	genBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	genBox.SetMarginTop(8)
	genBox.SetMarginStart(8)
	genBox.SetMarginEnd(8)
	genBox.SetMarginBottom(8)
	// currency
	currH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	currL, _ := gtk.LabelNew("Currency symbol:")
	currL.SetHAlign(gtk.ALIGN_START)
	currE, _ := gtk.EntryNew()
	currE.SetText(a.Tariff.Currency)
	currE.SetWidthChars(6)
	currH.PackStart(currL, false, false, 0)
	currH.PackStart(currE, false, false, 0)
	genBox.PackStart(currH, false, false, 0)

	// default price
	priceH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	priceL, _ := gtk.LabelNew("Default price (per kWh):")
	priceL.SetHAlign(gtk.ALIGN_START)
	priceAdj, _ := gtk.AdjustmentNew(a.Tariff.DefaultPrice, 0, 10, 0.01, 0.1, 0)
	priceSpin, _ := gtk.SpinButtonNew(priceAdj, 0.01, 3)
	priceSpin.SetDigits(3)
	priceH.PackStart(priceL, false, false, 0)
	priceH.PackStart(priceSpin, false, false, 0)
	genBox.PackStart(priceH, false, false, 0)

	// frameless
	frameChk, _ := gtk.CheckButtonNewWithLabel("Frameless window (no title bar, drag via header)")
	frameChk.SetActive(a.Config.Frameless)
	genBox.PackStart(frameChk, false, false, 0)
	topChk, _ := gtk.CheckButtonNewWithLabel("Always on top")
	topChk.SetActive(a.Config.AlwaysOnTop)
	genBox.PackStart(topChk, false, false, 0)
	legendChk, _ := gtk.CheckButtonNewWithLabel("Show legend bar (colours + total)")
	legendChk.SetActive(a.Config.ShowLegend)
	legendChk.Connect("toggled", func() {
		a.SetShowLegend(legendChk.GetActive())
	})
	genBox.PackStart(legendChk, false, false, 0)

	// RAPL status
	raplBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	raplBox.SetMarginTop(12)
	raplTitle, _ := gtk.LabelNew("")
	raplTitle.SetMarkup("<b>CPU Power (RAPL)</b>")
	raplTitle.SetHAlign(gtk.ALIGN_START)
	raplBox.PackStart(raplTitle, false, false, 0)
	samp := sampler.New()
	hasRapl := samp.HasRapl()
	accessible := samp.RaplAccessible()
	statusTxt := ""
	if !hasRapl {
		statusTxt = "No RAPL zones found (this CPU may not support RAPL)"
	} else if accessible {
		statusTxt = "✔ Accessible (package-0, core)"
	} else {
		statusTxt = "✖ Permission denied — needs udev rule"
	}
	raplLbl, _ := gtk.LabelNew(statusTxt)
	raplLbl.SetHAlign(gtk.ALIGN_START)
	raplBox.PackStart(raplLbl, false, false, 0)
	if hasRapl && !accessible {
		fixBtn, _ := gtk.ButtonNewWithLabel("Fix Permissions (prompt for sudo password)")
		raplBox.PackStart(fixBtn, false, false, 0)
		fixBtn.Connect("clicked", func() {
			go func() {
				// pkexec script
				script := "cp " + appDir() + "/etc/99-rapl.rules /etc/udev/rules.d/99-rapl.rules && chmod 644 /etc/udev/rules.d/99-rapl.rules && udevadm control --reload-rules && udevadm trigger --subsystem-match=powercap && chmod 444 /sys/class/powercap/intel-rapl*/energy_uj 2>/dev/null; true"
				cmd := exec.Command("pkexec", "sh", "-c", script)
				out, err := cmd.CombinedOutput()
				glib.IdleAdd(func() {
					if err != nil {
						dlg := gtk.MessageDialogNew(dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, "Failed to fix RAPL: %v\n%s\n\nManual fix:\nsudo cp etc/99-rapl.rules /etc/udev/rules.d/ && sudo udevadm trigger", err, string(out))
						dlg.Run()
						dlg.Destroy()
					} else {
						raplLbl.SetText("✔ Fixed — restart wattch or wait a moment for permissions to apply")
					}
				})
			}()
		})
	}
	genBox.PackStart(raplBox, false, false, 0)

	// reset cost
	resetBtn, _ := gtk.ButtonNewWithLabel("Reset accumulated cost")
	resetBtn.Connect("clicked", func() {
		confirm := gtk.MessageDialogNew(dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO, "Reset cost and energy counters to zero?")
		resp := confirm.Run()
		confirm.Destroy()
		if resp == gtk.RESPONSE_YES {
			a.Accum.Reset(time.Now())
			_ = config.SaveState(a.Accum.State)
			a.CostLabel.SetText(fmt.Sprintf("%s%.2f since %s", a.Tariff.Currency, 0.0, time.Now().Format("2006-01-02 15:04")))
		}
	})
	genBox.PackStart(resetBtn, false, false, 0)

	genLbl, _ := gtk.LabelNew("General")
	notebook.AppendPage(genBox, genLbl)

	// Tariff tab
	tariffBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	tariffBox.SetMarginTop(8)
	tariffBox.SetMarginStart(8)
	tariffBox.SetMarginEnd(8)
	tariffBox.SetMarginBottom(8)
	helpLbl, _ := gtk.LabelNew("Rules are evaluated top-first. First matching day+time wins, else Default. Empty list = default only.")
	helpLbl.SetLineWrap(true)
	helpLbl.SetHAlign(gtk.ALIGN_START)
	tariffBox.PackStart(helpLbl, false, false, 0)

	// listbox
	scrolled, _ := gtk.ScrolledWindowNew(nil, nil)
	scrolled.SetVExpand(true)
	scrolled.SetSizeRequest(-1, 220)
	listBox, _ := gtk.ListBoxNew()
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)
	scrolled.Add(listBox)
	tariffBox.PackStart(scrolled, true, true, 0)

	// helper to rebuild list
	var rebuild func()
	rebuild = func() {
		// clear
		for {
			row := listBox.GetRowAtIndex(0)
			if row == nil {
				break
			}
			listBox.Remove(row)
		}
		for idx, r := range a.Tariff.Rules {
			row, _ := gtk.ListBoxRowNew()
			hb, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
			hb.SetMarginTop(4)
			hb.SetMarginBottom(4)
			hb.SetMarginStart(6)
			hb.SetMarginEnd(6)
			lbl, _ := gtk.LabelNew("")
			days := strings.Join(r.Days, " ")
			if len(r.Days) == 0 {
				days = "All days"
			}
			timeStr := "all-day"
			if r.Start != "" || r.End != "" {
				timeStr = r.Start + "–" + r.End
			}
			lbl.SetMarkup(fmt.Sprintf("<b>%s</b>  %s  %s  %.3f", r.Label, days, timeStr, r.Price))
			lbl.SetHAlign(gtk.ALIGN_START)
			lbl.SetHExpand(true)
			hb.PackStart(lbl, true, true, 0)
			// up/down
			upBtn, _ := gtk.ButtonNewWithLabel("↑")
			upBtn.SetTooltipText("Move up")
			downBtn, _ := gtk.ButtonNewWithLabel("↓")
			downBtn.SetTooltipText("Move down")
			editBtn, _ := gtk.ButtonNewWithLabel("Edit")
			delBtn, _ := gtk.ButtonNewWithLabel("Del")
			hb.PackStart(upBtn, false, false, 0)
			hb.PackStart(downBtn, false, false, 0)
			hb.PackStart(editBtn, false, false, 0)
			hb.PackStart(delBtn, false, false, 0)
			// capture idx
			i := idx
			upBtn.Connect("clicked", func() {
				if i > 0 {
					a.Tariff.Rules[i], a.Tariff.Rules[i-1] = a.Tariff.Rules[i-1], a.Tariff.Rules[i]
					_ = config.SaveTariff(a.Tariff)
					rebuild()
				}
			})
			downBtn.Connect("clicked", func() {
				if i < len(a.Tariff.Rules)-1 {
					a.Tariff.Rules[i], a.Tariff.Rules[i+1] = a.Tariff.Rules[i+1], a.Tariff.Rules[i]
					_ = config.SaveTariff(a.Tariff)
					rebuild()
				}
			})
			editBtn.Connect("clicked", func() {
				showRuleDialog(dialog, &a.Tariff.Rules[i], func() {
					_ = config.SaveTariff(a.Tariff)
					rebuild()
				})
			})
			delBtn.Connect("clicked", func() {
				// remove
				a.Tariff.Rules = append(a.Tariff.Rules[:i], a.Tariff.Rules[i+1:]...)
				_ = config.SaveTariff(a.Tariff)
				rebuild()
			})
			row.Add(hb)
			listBox.Add(row)
		}
		listBox.ShowAll()
	}
	rebuild()

	addBtn, _ := gtk.ButtonNewWithLabel("+ Add Rule")
	addBtn.Connect("clicked", func() {
		newRule := config.TariffRule{Label: "New rule", Days: []string{"Mon", "Tue", "Wed", "Thu", "Fri"}, Start: "07:00", End: "10:00", Price: a.Tariff.DefaultPrice}
		showRuleDialog(dialog, &newRule, func() {
			a.Tariff.Rules = append(a.Tariff.Rules, newRule)
			_ = config.SaveTariff(a.Tariff)
			rebuild()
		})
	})
	tariffBox.PackStart(addBtn, false, false, 0)

	tariffLbl, _ := gtk.LabelNew("Tariff")
	notebook.AppendPage(tariffBox, tariffLbl)

	// Graph tab
	graphBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	graphBox.SetMarginTop(8)
	graphBox.SetMarginStart(8)
	graphBox.SetMarginEnd(8)
	sampleH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	sampleL, _ := gtk.LabelNew("Sample interval (ms):")
	sampleAdj, _ := gtk.AdjustmentNew(float64(a.Config.SampleMs), 500, 5000, 100, 500, 0)
	sampleSpin, _ := gtk.SpinButtonNew(sampleAdj, 100, 0)
	sampleH.PackStart(sampleL, false, false, 0)
	sampleH.PackStart(sampleSpin, false, false, 0)
	graphBox.PackStart(sampleH, false, false, 0)

	histH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	histL, _ := gtk.LabelNew("History length (seconds):")
	histAdj, _ := gtk.AdjustmentNew(float64(a.Config.HistoryS), 60, 3600, 10, 60, 0)
	histSpin, _ := gtk.SpinButtonNew(histAdj, 10, 0)
	histH.PackStart(histL, false, false, 0)
	histH.PackStart(histSpin, false, false, 0)
	graphBox.PackStart(histH, false, false, 0)

	applyGraphBtn, _ := gtk.ButtonNewWithLabel("Apply (restart sampler)")
	applyGraphBtn.Connect("clicked", func() {
		a.Config.SampleMs = int(sampleSpin.GetValue())
		a.Config.HistoryS = int(histSpin.GetValue())
		_ = config.SaveConfig(a.Config)
		// resize history
		newMax := a.Config.HistoryS * 1000 / a.Config.SampleMs
		a.History.Resize(newMax)
		// restart ticker
		if a.Ticker != nil {
			a.Ticker.Stop()
		}
		// Note: we don't restart poll goroutine here fully; ticker replacement needs new goroutine
		// For v1, just inform user restart required
		dlg := gtk.MessageDialogNew(dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_OK, "Restart wattch to apply new interval/history.")
		dlg.Run()
		dlg.Destroy()
	})
	graphBox.PackStart(applyGraphBtn, false, false, 0)
	graphLbl, _ := gtk.LabelNew("Graph")
	notebook.AppendPage(graphBox, graphLbl)

	// save on close
	dialog.Connect("response", func() {
		// save general
		if txt, err := currE.GetText(); err == nil {
			a.Tariff.Currency = txt
		}
		if a.Tariff.Currency == "" {
			a.Tariff.Currency = "$"
		}
		a.Tariff.DefaultPrice = priceSpin.GetValue()
		a.Config.Frameless = frameChk.GetActive()
		a.Config.AlwaysOnTop = topChk.GetActive()
		a.Config.ShowLegend = legendChk.GetActive()
		_ = config.SaveTariff(a.Tariff)
		_ = config.SaveConfig(a.Config)
		// apply window decor + legend
		a.Window.SetDecorated(!a.Config.Frameless)
		a.Window.SetKeepAbove(a.Config.AlwaysOnTop)
		a.SetShowLegend(a.Config.ShowLegend)
		a.Config.Currency = a.Tariff.Currency
		_ = config.SaveConfig(a.Config)
		dialog.Destroy()
	})

	dialog.ShowAll()
	_ = rebuild
}

func showRuleDialog(parent *gtk.Dialog, rule *config.TariffRule, onSave func()) {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Edit Rule")
	dlg.SetTransientFor(parent)
	dlg.SetModal(true)
	dlg.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dlg.AddButton("Save", gtk.RESPONSE_OK)
	dlg.SetDefaultSize(420, 300)
	box, _ := dlg.GetContentArea()
	vb, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	vb.SetMarginTop(8)
	vb.SetMarginStart(8)
	vb.SetMarginEnd(8)
	box.PackStart(vb, true, true, 0)

	// label
	lbH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	lbL, _ := gtk.LabelNew("Label:")
	le, _ := gtk.EntryNew()
	le.SetText(rule.Label)
	lbH.PackStart(lbL, false, false, 0)
	lbH.PackStart(le, true, true, 0)
	vb.PackStart(lbH, false, false, 0)

	// days
	daysL, _ := gtk.LabelNew("Days:")
	daysL.SetHAlign(gtk.ALIGN_START)
	vb.PackStart(daysL, false, false, 0)
	daysBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	checks := make([]*gtk.CheckButton, 7)
	activeMap := map[string]bool{}
	for _, d := range rule.Days {
		activeMap[d] = true
	}
	allEmptyMeansAll := len(rule.Days) == 0
	for i, dn := range dayNames {
		cb, _ := gtk.CheckButtonNewWithLabel(dn)
		if allEmptyMeansAll {
			// show all checked if empty? leave unchecked to mean all? we show unchecked
			cb.SetActive(false)
		} else {
			cb.SetActive(activeMap[dn])
		}
		daysBox.PackStart(cb, false, false, 0)
		checks[i] = cb
	}
	vb.PackStart(daysBox, false, false, 0)
	hint, _ := gtk.LabelNew("Leave all unchecked = every day")
	hint.SetHAlign(gtk.ALIGN_START)
	vb.PackStart(hint, false, false, 0)

	// time
	timeH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	startL, _ := gtk.LabelNew("Start HH:MM:")
	endL, _ := gtk.LabelNew("End HH:MM:")
	startE, _ := gtk.EntryNew()
	startE.SetText(rule.Start)
	startE.SetWidthChars(5)
	endE, _ := gtk.EntryNew()
	endE.SetText(rule.End)
	endE.SetWidthChars(5)
	timeH.PackStart(startL, false, false, 0)
	timeH.PackStart(startE, false, false, 0)
	timeH.PackStart(endL, false, false, 0)
	timeH.PackStart(endE, false, false, 0)
	vb.PackStart(timeH, false, false, 0)
	allDayHint, _ := gtk.LabelNew("Leave both empty = all-day")
	allDayHint.SetHAlign(gtk.ALIGN_START)
	vb.PackStart(allDayHint, false, false, 0)

	// price
	priceH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	priceL, _ := gtk.LabelNew("Price per kWh:")
	priceAdj, _ := gtk.AdjustmentNew(rule.Price, 0, 10, 0.001, 0.01, 0)
	priceSpin, _ := gtk.SpinButtonNew(priceAdj, 0.001, 3)
	priceSpin.SetDigits(3)
	priceH.PackStart(priceL, false, false, 0)
	priceH.PackStart(priceSpin, false, false, 0)
	vb.PackStart(priceH, false, false, 0)

	dlg.ShowAll()
	resp := dlg.Run()
	if resp == gtk.RESPONSE_OK {
		if txt, err := le.GetText(); err == nil {
			rule.Label = txt
		}
		if rule.Label == "" {
			rule.Label = "Unnamed"
		}
		var days []string
		anyChecked := false
		for i, cb := range checks {
			if cb.GetActive() {
				days = append(days, dayNames[i])
				anyChecked = true
			}
		}
		if !anyChecked {
			days = []string{} // all days
		}
		rule.Days = days
		if s, err := startE.GetText(); err == nil {
			rule.Start = strings.TrimSpace(s)
		}
		if e, err := endE.GetText(); err == nil {
			rule.End = strings.TrimSpace(e)
		}
		// validate HH:MM empty or correct
		if rule.Start != "" {
			if _, ok := parseHM(rule.Start); !ok {
				// keep but warn? just keep
			}
		}
		if rule.End != "" {
			if _, ok := parseHM(rule.End); !ok {
			}
		}
		// price
		rule.Price, _ = strconv.ParseFloat(fmt.Sprintf("%.3f", priceSpin.GetValue()), 64)
		if onSave != nil {
			onSave()
		}
	}
	dlg.Destroy()
}

func parseHM(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func appDir() string {
	return "/home/rj/src/wattch"
}
