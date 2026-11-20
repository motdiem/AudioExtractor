package window

import "testing"

const w, h = 420, 320

// A laptop screen with a menu bar, and an external monitor to its right.
var (
	laptop  = Rect{X: 0, Y: 25, Width: 1440, Height: 875}
	monitor = Rect{X: 1440, Y: -200, Width: 2560, Height: 1440}
)

func TestPlace(t *testing.T) {
	tests := []struct {
		name  string
		pos   Point
		areas []Rect
		want  Point
	}{
		{"visible position is kept", Point{100, 200}, []Rect{laptop}, Point{100, 200}},
		{"on the second monitor", Point{2000, 300}, []Rect{laptop, monitor}, Point{2000, 300}},
		{"hanging off an edge, title bar visible", Point{1200, 600}, []Rect{laptop}, Point{1200, 600}},
		{"monitor unplugged: back onto the laptop", Point{2000, 300}, []Rect{laptop}, Point{1440 - w, 300}},
		{"above the screen", Point{100, -500}, []Rect{laptop}, Point{100, 25}},
		{"below the screen", Point{100, 5000}, []Rect{laptop}, Point{100, 900 - h}},
		{"far left: nearest screen wins", Point{-3000, 100}, []Rect{monitor, laptop}, Point{0, 100}},
		{"title bar under the menu bar", Point{100, 0}, []Rect{laptop}, Point{100, 25}},
		{"no screen information", Point{-9000, -9000}, nil, Point{-9000, -9000}},
		{"screen smaller than the window", Point{500, 500}, []Rect{{0, 0, 300, 200}}, Point{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Place(tt.pos, w, h, tt.areas); got != tt.want {
				t.Errorf("Place(%v) = %v, want %v", tt.pos, got, tt.want)
			}
		})
	}
}
