package ui

import (
	"fmt"
	"math"

	"github.com/gotk3/gotk3/cairo"
	"github.com/ScratchingMyHead/wattch/internal/sampler"
)

type History struct {
	MaxPoints int
	Samples   []sampler.Sample
	// per source max for scaling? we compute dynamic
}

func NewHistory(maxPoints int) *History {
	return &History{MaxPoints: maxPoints, Samples: make([]sampler.Sample, 0, maxPoints)}
}

func (h *History) Add(s sampler.Sample) {
	if len(h.Samples) >= h.MaxPoints && h.MaxPoints > 0 {
		// drop oldest
		copy(h.Samples, h.Samples[1:])
		h.Samples = h.Samples[:h.MaxPoints-1]
	}
	h.Samples = append(h.Samples, s)
}

func (h *History) Resize(newMax int) {
	h.MaxPoints = newMax
	if len(h.Samples) > newMax {
		h.Samples = h.Samples[len(h.Samples)-newMax:]
	}
}

// palette similar to energraph hues
var palette = []string{
	"#e74c3c", "#3498db", "#2ecc71", "#f1c40f", "#9b59b6",
	"#1abc9c", "#e67e22", "#34495e", "#ff6b6b", "#4dabf7",
}

func colorFor(source string, idx int) (r, g, b float64) {
	// idx based
	c := palette[idx%len(palette)]
	return hexToRGB(c)
}

func hexToRGB(h string) (float64, float64, float64) {
	if len(h) != 7 || h[0] != '#' {
		return 1, 1, 1
	}
	var r, g, b int
	// parse hex
	for i := 0; i < 2; i++ {
		c := h[1+i]
		v := 0
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			v = int(c-'a') + 10
		} else if c >= 'A' && c <= 'F' {
			v = int(c-'A') + 10
		}
		r = r*16 + v
	}
	for i := 0; i < 2; i++ {
		c := h[3+i]
		v := 0
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			v = int(c-'a') + 10
		} else if c >= 'A' && c <= 'F' {
			v = int(c-'A') + 10
		}
		g = g*16 + v
	}
	for i := 0; i < 2; i++ {
		c := h[5+i]
		v := 0
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			v = int(c-'a') + 10
		} else if c >= 'A' && c <= 'F' {
			v = int(c-'A') + 10
		}
		b = b*16 + v
	}
	return float64(r) / 255.0, float64(g) / 255.0, float64(b) / 255.0
}

func DrawHistory(cr *cairo.Context, w, h int, hist *History, colors map[string]string, orderedSources []string) {
	// background
	cr.SetSourceRGB(0.12, 0.12, 0.12)
	cr.Rectangle(0, 0, float64(w), float64(h))
	cr.Fill()

	if len(hist.Samples) < 2 {
		return
	}
	// find max
	maxW := 50.0
	for _, s := range hist.Samples {
		if s.Total > maxW {
			maxW = s.Total
		}
		for _, v := range s.Sources {
			if v > maxW {
				maxW = v
			}
		}
	}
	maxW *= 1.1
	if maxW < 50 {
		maxW = 50
	}
	// grid
	cr.SetSourceRGB(0.25, 0.25, 0.25)
	cr.SetLineWidth(0.5)
	for i := 1; i < 4; i++ {
		y := float64(h) * float64(i) / 4.0
		cr.MoveTo(0, y)
		cr.LineTo(float64(w), y)
		cr.Stroke()
	}
	// determine source order: use orderedSources if provided else map keys
	sourceList := orderedSources
	if len(sourceList) == 0 {
		// collect
		set := map[string]bool{}
		for _, s := range hist.Samples {
			for k := range s.Sources {
				set[k] = true
			}
		}
		for k := range set {
			sourceList = append(sourceList, k)
		}
	}
	// draw per-source lines
	for idx, src := range sourceList {
		var r, g, b float64
		if hex, ok := colors[src]; ok && hex != "" {
			r, g, b = hexToRGB(hex)
		} else {
			r, g, b = colorFor(src, idx)
		}
		cr.SetSourceRGB(r, g, b)
		cr.SetLineWidth(1.2)
		first := true
		for i, s := range hist.Samples {
			v, ok := s.Sources[src]
			if !ok {
				v = 0
			}
			x := float64(i) / float64(max(1, hist.MaxPoints-1)) * float64(w)
			// if less samples than max, stretch
			if len(hist.Samples) < hist.MaxPoints {
				x = float64(i) / float64(max(1, len(hist.Samples)-1)) * float64(w)
			}
			y := float64(h) - (v/maxW)*float64(h)
			y = math.Max(0, math.Min(float64(h), y))
			if first {
				cr.MoveTo(x, y)
				first = false
			} else {
				cr.LineTo(x, y)
			}
		}
		cr.Stroke()
	}
	// draw total in white
	cr.SetSourceRGB(1, 1, 1)
	cr.SetLineWidth(1.5)
	first := true
	for i, s := range hist.Samples {
		x := float64(i) / float64(max(1, hist.MaxPoints-1)) * float64(w)
		if len(hist.Samples) < hist.MaxPoints {
			x = float64(i) / float64(max(1, len(hist.Samples)-1)) * float64(w)
		}
		y := float64(h) - (s.Total/maxW)*float64(h)
		if first {
			cr.MoveTo(x, y)
			first = false
		} else {
			cr.LineTo(x, y)
		}
	}
	cr.Stroke()
	// Y labels
	cr.SetSourceRGB(0.8, 0.8, 0.8)
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(9)
	cr.MoveTo(4, 12)
	cr.ShowText(formatW(maxW))
	cr.MoveTo(4, float64(h)-4)
	cr.ShowText("0W")
}

func formatW(v float64) string {
	if v >= 1000 {
		return fmt.Sprintf("%.1fkW", v/1000)
	}
	return fmt.Sprintf("%.0fW", v)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
