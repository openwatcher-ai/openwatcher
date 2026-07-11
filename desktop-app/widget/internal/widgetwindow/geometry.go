package widgetwindow

import "math"

const (
	Orb               = 56.0
	Margin            = 12.0
	CornerTopLeft     = "top-left"
	CornerTopRight    = "top-right"
	CornerBottomLeft  = "bottom-left"
	CornerBottomRight = "bottom-right"
)

type Point struct{ X, Y float64 }
type WorkArea struct {
	X, Y, Width, Height float64
	MonitorID           string
}
type Geometry struct {
	X, Y, Width, Height float64
	Expanded            bool
}
type SavedPosition struct {
	MonitorID, Edge string
	Normalized      float64
}

func DefaultGeometry() Geometry { return Geometry{Width: Orb, Height: Orb} }
func Clamp(g Geometry, a WorkArea, pad float64) Geometry {
	if a.Width <= 0 || a.Height <= 0 {
		return g
	}
	maxW, maxH := math.Max(Orb, a.Width-2*pad), math.Max(Orb, a.Height-2*pad)
	if g.Width > maxW {
		g.Width = maxW
	}
	if g.Height > maxH {
		g.Height = maxH
	}
	g.X = math.Max(a.X+pad, math.Min(g.X, a.X+a.Width-pad-g.Width))
	g.Y = math.Max(a.Y+pad, math.Min(g.Y, a.Y+a.Height-pad-g.Height))
	return g
}
func PanelSize(a WorkArea) (float64, float64) {
	w, h := 1120.0, 580.0
	if a.Width < 1280 || a.Height < 720 {
		w, h = 920, 500
	}
	w = math.Min(w, math.Max(Orb, a.Width-2*Margin))
	h = math.Min(h, math.Max(Orb, a.Height-2*Margin))
	return w, h
}
func Resize(g Geometry, expanded bool, a WorkArea) Geometry {
	g.Expanded = expanded
	if !expanded {
		g.Width, g.Height = Orb, Orb
		return Clamp(g, a, Margin)
	}
	g.Width, g.Height = PanelSize(a)
	return Clamp(g, a, Margin)
}

// AnchorCorner chooses the panel corner that can preserve the orb's current
// screen-edge position while placing most of the panel inside the work area.
func AnchorCorner(anchor Geometry, a WorkArea) string {
	horizontal := "left"
	vertical := "top"
	if anchor.X+anchor.Width/2 >= a.X+a.Width/2 {
		horizontal = "right"
	}
	if anchor.Y+anchor.Height/2 >= a.Y+a.Height/2 {
		vertical = "bottom"
	}
	return vertical + "-" + horizontal
}

// ExpandAt grows the panel inward from the collapsed orb's saved corner. The
// orb is hidden while expanded, and the corner maps the panel back to the
// original collapsed position.
func ExpandAt(anchor Geometry, a WorkArea, corner string) Geometry {
	w, h := PanelSize(a)
	g := Geometry{Width: w, Height: h, Expanded: true}
	switch corner {
	case CornerTopLeft:
		g.X, g.Y = anchor.X, anchor.Y
	case CornerTopRight:
		g.X, g.Y = anchor.X-w+Orb, anchor.Y
	case CornerBottomLeft:
		g.X, g.Y = anchor.X, anchor.Y-h+Orb
	default:
		g.X, g.Y = anchor.X-w+Orb, anchor.Y-h+Orb
	}
	return Clamp(g, a, Margin)
}

// AnchorFromPanel converts the displayed panel rectangle back to the orb
// rectangle used for edge snapping and persistence.
func AnchorFromPanel(panel Geometry, corner string) Geometry {
	anchor := Geometry{Width: Orb, Height: Orb}
	switch corner {
	case CornerTopLeft:
		anchor.X, anchor.Y = panel.X, panel.Y
	case CornerTopRight:
		anchor.X, anchor.Y = panel.X+panel.Width-Orb, panel.Y
	case CornerBottomLeft:
		anchor.X, anchor.Y = panel.X, panel.Y+panel.Height-Orb
	default:
		anchor.X = panel.X + panel.Width - Orb
		anchor.Y = panel.Y + panel.Height - Orb
	}
	return anchor
}
func Snap(p Point, a WorkArea, size, pad float64) Geometry {
	// The nearest of all four edges wins, not just left/right.
	d := []float64{math.Abs(p.X - a.X), math.Abs(a.X + a.Width - (p.X + size)), math.Abs(p.Y - a.Y), math.Abs(a.Y + a.Height - (p.Y + size))}
	edge := 0
	for i := 1; i < len(d); i++ {
		if d[i] < d[edge] {
			edge = i
		}
	}
	g := Geometry{X: p.X, Y: p.Y, Width: size, Height: size}
	switch edge {
	case 0:
		g.X = a.X + pad
	case 1:
		g.X = a.X + a.Width - size - pad
	case 2:
		g.Y = a.Y + pad
	case 3:
		g.Y = a.Y + a.Height - size - pad
	}
	return Clamp(g, a, pad)
}
func Save(g Geometry, a WorkArea) SavedPosition {
	d := []float64{math.Abs(g.X - a.X), math.Abs(a.X + a.Width - (g.X + g.Width)), math.Abs(g.Y - a.Y), math.Abs(a.Y + a.Height - (g.Y + g.Height))}
	edge := 0
	for i := 1; i < 4; i++ {
		if d[i] < d[edge] {
			edge = i
		}
	}
	p := SavedPosition{MonitorID: a.MonitorID}
	if edge < 2 {
		p.Normalized = (g.Y - a.Y) / math.Max(1, a.Height-g.Height)
		if edge == 0 {
			p.Edge = "left"
		} else {
			p.Edge = "right"
		}
	} else {
		p.Normalized = (g.X - a.X) / math.Max(1, a.Width-g.Width)
		if edge == 2 {
			p.Edge = "top"
		} else {
			p.Edge = "bottom"
		}
	}
	p.Normalized = math.Max(0, math.Min(1, p.Normalized))
	return p
}
func Restore(p SavedPosition, areas []WorkArea) Geometry {
	if len(areas) == 0 {
		return DefaultGeometry()
	}
	a := areas[0]
	for _, candidate := range areas {
		if candidate.MonitorID == p.MonitorID {
			a = candidate
			break
		}
	}
	g := DefaultGeometry()
	n := math.Max(0, math.Min(1, p.Normalized))
	switch p.Edge {
	case "right":
		g.X = a.X + a.Width - Orb - Margin
		g.Y = a.Y + n*(a.Height-Orb)
	case "top":
		g.X = a.X + n*(a.Width-Orb)
		g.Y = a.Y + Margin
	case "bottom":
		g.X = a.X + n*(a.Width-Orb)
		g.Y = a.Y + a.Height - Orb - Margin
	default:
		g.X = a.X + Margin
		g.Y = a.Y + n*(a.Height-Orb)
	}
	return Clamp(g, a, Margin)
}
