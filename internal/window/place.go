// Package window remembers where the main window was and makes it follow the
// system light/dark theme. The placement rule (Place) is pure and tested; the
// Wails glue lives in memory.go and theme.go.
package window

// Rect is a rectangle in desktop coordinates: DIPs (points on macOS), origin
// at the top-left of the primary screen, y growing downwards. That is the
// space Wails uses both for a window's Position and for a Screen's WorkArea.
type Rect struct{ X, Y, Width, Height int }

// Point is a window's top-left corner, as saved in the settings file.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// grip is the height of the strip at the top of the window (the title bar)
// whose middle must stay on a screen for the user to be able to drag the
// window back.
const grip = 30

// Place returns where a window of the given size whose saved top-left corner
// is pos should open, given the work areas of the screens connected now.
//
// A position whose title bar is still on a screen is kept as is, even if part
// of the window hangs off an edge: the user put it there. Otherwise (the
// monitor it was on is gone, or the resolution shrank) the window moves to the
// nearest screen, fully inside its work area. With no screen information the
// position is returned unchanged.
func Place(pos Point, width, height int, areas []Rect) Point {
	if len(areas) == 0 {
		return pos
	}
	handle := Point{pos.X + width/2, pos.Y + grip/2}
	for _, a := range areas {
		if a.contains(handle) {
			return pos
		}
	}
	best := areas[0]
	bestDist := best.distance2(handle)
	for _, a := range areas[1:] {
		if d := a.distance2(handle); d < bestDist {
			best, bestDist = a, d
		}
	}
	return Point{
		X: clamp(pos.X, best.X, best.X+best.Width-width),
		Y: clamp(pos.Y, best.Y, best.Y+best.Height-height),
	}
}

func (r Rect) contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// distance2 is the squared distance from p to the nearest point of r.
func (r Rect) distance2(p Point) int {
	dx := max(r.X-p.X, 0, p.X-(r.X+r.Width-1))
	dy := max(r.Y-p.Y, 0, p.Y-(r.Y+r.Height-1))
	return dx*dx + dy*dy
}

// clamp keeps v in [lo, hi]; when the window is larger than the area
// (hi < lo), its top-left corner wins so the title bar stays reachable.
func clamp(v, lo, hi int) int {
	if v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return v
}
