package widgetwindow

import "testing"

func TestResizeUsesCompactSizeAndWorkAreaOrigin(t *testing.T) {
	a := WorkArea{X: 100, Y: 40, Width: 1000, Height: 700, MonitorID: "one"}
	g := Resize(DefaultGeometry(), true, a)
	if g.Width != 920 || g.Height != 500 || g.X < a.X+Margin || g.Y < a.Y+Margin {
		t.Fatalf("%+v", g)
	}
}
func TestSnapSupportsAllFourEdges(t *testing.T) {
	a := WorkArea{X: 500, Y: 100, Width: 1000, Height: 700}
	for _, point := range []Point{{510, 350}, {1435, 350}, {900, 105}, {900, 735}} {
		g := Snap(point, a, Orb, 8)
		if g.X < a.X || g.Y < a.Y || g.X+g.Width > a.X+a.Width || g.Y+g.Height > a.Y+a.Height {
			t.Fatalf("off screen: %+v", g)
		}
	}
}
func TestRestoreFallsBackToPrimaryMonitor(t *testing.T) {
	areas := []WorkArea{{X: 0, Y: 0, Width: 800, Height: 600, MonitorID: "primary"}}
	g := Restore(SavedPosition{MonitorID: "removed", Edge: "right", Normalized: .8}, areas)
	if g.X+g.Width > 800 || g.Y < 0 {
		t.Fatalf("%+v", g)
	}
}
func TestSaveRestorePreservesEdgePosition(t *testing.T) {
	a := WorkArea{X: 100, Y: 50, Width: 1200, Height: 800, MonitorID: "wide"}
	p := Save(Geometry{X: 100 + Margin, Y: 470, Width: Orb, Height: Orb}, a)
	if p.Edge != "left" {
		t.Fatalf("%+v", p)
	}
	g := Restore(p, []WorkArea{a})
	if g.X != a.X+Margin {
		t.Fatalf("%+v", g)
	}
}

func TestExpandAtPreservesAllFourAnchorCorners(t *testing.T) {
	a := WorkArea{X: -900, Y: 40, Width: 1200, Height: 800, MonitorID: "left"}
	anchors := []Geometry{
		{X: a.X + Margin, Y: a.Y + Margin, Width: Orb, Height: Orb},
		{X: a.X + a.Width - Orb - Margin, Y: a.Y + Margin, Width: Orb, Height: Orb},
		{X: a.X + Margin, Y: a.Y + a.Height - Orb - Margin, Width: Orb, Height: Orb},
		{X: a.X + a.Width - Orb - Margin, Y: a.Y + a.Height - Orb - Margin, Width: Orb, Height: Orb},
	}
	for _, anchor := range anchors {
		corner := AnchorCorner(anchor, a)
		panel := ExpandAt(anchor, a, corner)
		got := AnchorFromPanel(panel, corner)
		if got.X != anchor.X || got.Y != anchor.Y {
			t.Fatalf("%s: got %+v, want %+v", corner, got, anchor)
		}
	}
}
