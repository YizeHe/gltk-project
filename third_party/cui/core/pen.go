package core

import "cui/win32"

// Pen represents a GDI pen for drawing lines and borders
type Pen struct {
	Handle win32.HPEN
}

// NewPen creates a pen with the given style, width, and color
func NewPen(style int32, width int32, color Color) *Pen {
	h := win32.CreatePen(style, width, color.ToCOLORREF())
	TrackGDI(win32.HGDIOBJ(h), 0, "pen")
	return &Pen{Handle: h}
}

// NewSolidPen creates a solid pen
func NewSolidPen(width int32, color Color) *Pen {
	return NewPen(win32.PS_SOLID, width, color)
}

// Dispose releases the pen resource
func (p *Pen) Dispose() {
	if p.Handle != 0 {
		win32.DeleteObject(win32.HGDIOBJ(p.Handle))
		p.Handle = 0
	}
}

// StockPen returns a stock pen
func StockPen(index int32) *Pen {
	return &Pen{Handle: win32.HPEN(win32.GetStockObject(index))}
}
