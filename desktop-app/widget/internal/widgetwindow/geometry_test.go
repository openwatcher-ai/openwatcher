package widgetwindow

import "testing"

func TestResizeAndClamp(t *testing.T) {
	g := Resize(DefaultGeometry(), true, 1000, 700)
	g = Clamp(g, WorkArea{1000, 700}, 12)
	if g.Width != 976 || g.Height != 580 {
		t.Fatalf("%+v", g)
	}
}
func TestSnap(t *testing.T) {
	g := Snap(Point{900, 100}, WorkArea{1000, 700}, 56, 8)
	if g.X != 936 {
		t.Fatal(g.X)
	}
}
