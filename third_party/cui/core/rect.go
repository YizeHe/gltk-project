package core

import "cui/win32"

// Rect represents a rectangle with integer coordinates
type Rect struct {
	X, Y, Width, Height int32
}

// NewRect creates a new Rect
func NewRect(x, y, w, h int32) Rect {
	return Rect{X: x, Y: y, Width: w, Height: h}
}

// Left returns the left edge
func (r Rect) Left() int32 { return r.X }

// Top returns the top edge
func (r Rect) Top() int32 { return r.Y }

// Right returns the right edge
func (r Rect) Right() int32 { return r.X + r.Width }

// Bottom returns the bottom edge
func (r Rect) Bottom() int32 { return r.Y + r.Height }

// Contains checks if a point is inside the rectangle
func (r Rect) Contains(x, y int32) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// ToRECT converts to win32.RECT
func (r Rect) ToRECT() win32.RECT {
	return win32.RECT{
		Left:   win32.LONG(r.X),
		Top:    win32.LONG(r.Y),
		Right:  win32.LONG(r.X + r.Width),
		Bottom: win32.LONG(r.Y + r.Height),
	}
}

// ToRECTPtr converts to *win32.RECT
func (r Rect) ToRECTPtr() *win32.RECT {
	v := r.ToRECT()
	return &v
}

// FromRECT converts from win32.RECT
func FromRECT(rc win32.RECT) Rect {
	return Rect{
		X:      int32(rc.Left),
		Y:      int32(rc.Top),
		Width:  int32(rc.Right - rc.Left),
		Height: int32(rc.Bottom - rc.Top),
	}
}

// Inset returns a rect inset by the given amounts
func (r Rect) Inset(left, top, right, bottom int32) Rect {
	return Rect{
		X:      r.X + left,
		Y:      r.Y + top,
		Width:  r.Width - left - right,
		Height: r.Height - top - bottom,
	}
}

// CenterIn centers this rect within the given outer rect
func (r Rect) CenterIn(outer Rect) Rect {
	return Rect{
		X:      outer.X + (outer.Width-r.Width)/2,
		Y:      outer.Y + (outer.Height-r.Height)/2,
		Width:  r.Width,
		Height: r.Height,
	}
}

// Move moves the rectangle to a new position
func (r Rect) Move(x, y int32) Rect {
	return Rect{X: x, Y: y, Width: r.Width, Height: r.Height}
}

// Size represents dimensions
type Size struct {
	Width, Height int32
}

// Point represents a 2D point
type Point struct {
	X, Y int32
}

// FromPOINT converts from win32.POINT
func FromPOINT(pt win32.POINT) Point {
	return Point{X: int32(pt.X), Y: int32(pt.Y)}
}

// ToPOINT converts to win32.POINT
func (p Point) ToPOINT() win32.POINT {
	return win32.POINT{X: win32.LONG(p.X), Y: win32.LONG(p.Y)}
}
