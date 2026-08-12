package core

import "cui/win32"

// Brush represents a GDI brush for filling shapes
type Brush struct {
	Handle win32.HBRUSH
}

// NewSolidBrush creates a brush with a solid color
func NewSolidBrush(color Color) *Brush {
	h := win32.CreateSolidBrush(color.ToCOLORREF())
	TrackGDI(win32.HGDIOBJ(h), 0, "brush")
	return &Brush{Handle: h}
}

// NewColorBrush is an alias for NewSolidBrush
func NewColorBrush(r, g, b uint8) *Brush {
	return NewSolidBrush(NewRGB(r, g, b))
}

// Dispose releases the brush resource
func (b *Brush) Dispose() {
	if b.Handle != 0 {
		win32.DeleteObject(win32.HGDIOBJ(b.Handle))
		b.Handle = 0
	}
}

// SystemBrush returns a stock brush
func SystemBrush(index int32) *Brush {
	return &Brush{Handle: win32.HBRUSH(win32.GetStockObject(index))}
}
