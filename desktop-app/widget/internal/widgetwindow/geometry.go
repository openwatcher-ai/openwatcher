package widgetwindow

type Point struct{ X, Y float64 }
type WorkArea struct{ Width, Height float64 }
type Geometry struct {
	X, Y, Width, Height float64
	Expanded            bool
}

func DefaultGeometry() Geometry { return Geometry{Width: 56, Height: 56} }
func Resize(g Geometry, expanded bool, screenW, screenH float64) Geometry {
	g.Expanded = expanded
	if expanded {
		g.Width, g.Height = 1120, 580
	} else {
		g.Width, g.Height = 56, 56
	}
	return Clamp(g, WorkArea{screenW, screenH}, 12)
}
func Clamp(g Geometry, a WorkArea, pad float64) Geometry {
	if a.Width <= 0 || a.Height <= 0 {
		return g
	}
	if g.Width > a.Width-2*pad {
		g.Width = a.Width - 2*pad
	}
	if g.Height > a.Height-2*pad {
		g.Height = a.Height - 2*pad
	}
	if g.X < pad {
		g.X = pad
	}
	if g.Y < pad {
		g.Y = pad
	}
	if g.X+g.Width > a.Width-pad {
		g.X = a.Width - pad - g.Width
	}
	if g.Y+g.Height > a.Height-pad {
		g.Y = a.Height - pad - g.Height
	}
	return g
}
func Snap(p Point, a WorkArea, size, pad float64) Geometry {
	g := Geometry{X: p.X, Y: p.Y, Width: size, Height: size}
	if p.X+size/2 < a.Width/2 {
		g.X = pad
	} else {
		g.X = a.Width - size - pad
	}
	if p.Y < pad {
		g.Y = pad
	}
	if p.Y+size > a.Height-pad {
		g.Y = a.Height - size - pad
	}
	return g
}
