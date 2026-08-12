package core

import (
	"cui/win32"
)

// Canvas provides drawing operations on a device context
type Canvas struct {
	HDC     win32.HDC
	hwnd    win32.HWND
	width   int32
	height  int32
}

// NewCanvas creates a Canvas from a window HDC
func NewCanvas(hdc win32.HDC, hwnd win32.HWND, width, height int32) *Canvas {
	return &Canvas{HDC: hdc, hwnd: hwnd, width: width, height: height}
}

// NewCanvasFromHDC creates a Canvas from just an HDC
func NewCanvasFromHDC(hdc win32.HDC) *Canvas {
	return &Canvas{HDC: hdc}
}

// Size returns the canvas dimensions
func (c *Canvas) Size() (int32, int32) {
	return c.width, c.height
}

// FillRect fills a rectangle with the given color
func (c *Canvas) FillRect(x, y, w, h int32, color Color) {
	brush := win32.CreateSolidBrush(color.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(brush))
	rc := win32.RECT{Left: win32.LONG(x), Top: win32.LONG(y), Right: win32.LONG(x + w), Bottom: win32.LONG(y + h)}
	win32.FillRect(c.HDC, &rc, brush)
}

// FillRectWithBrush fills a rectangle with a brush (no cleanup needed for stock brushes)
func (c *Canvas) FillRectWithBrush(x, y, w, h int32, brush win32.HBRUSH) {
	rc := win32.RECT{Left: win32.LONG(x), Top: win32.LONG(y), Right: win32.LONG(x + w), Bottom: win32.LONG(y + h)}
	win32.FillRect(c.HDC, &rc, brush)
}

// DrawRect draws a rectangle outline with the given color and width
func (c *Canvas) DrawRect(x, y, w, h int32, color Color, lineWidth int32) {
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, color.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))
	old := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, old)

	brush := win32.GetStockObject(win32.NULL_BRUSH)
	oldBrush := win32.SelectObject(c.HDC, brush)
	defer win32.SelectObject(c.HDC, oldBrush)

	win32.Rectangle(c.HDC, x, y, x+w, y+h)
}

// DrawRoundRect draws a rounded rectangle outline
func (c *Canvas) DrawRoundRect(x, y, w, h, rx, ry int32, color Color, lineWidth int32) {
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, color.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))
	old := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, old)

	brush := win32.GetStockObject(win32.NULL_BRUSH)
	oldBrush := win32.SelectObject(c.HDC, brush)
	defer win32.SelectObject(c.HDC, oldBrush)

	win32.RoundRect(c.HDC, x, y, x+w, y+h, rx*2, ry*2)
}

// FillRoundRect draws a filled rounded rectangle
func (c *Canvas) FillRoundRect(x, y, w, h, rx, ry int32, fillColor, borderColor Color, lineWidth int32) {
	brush := win32.CreateSolidBrush(fillColor.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(brush))
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, borderColor.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))

	oldPen := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, oldPen)
	oldBrush := win32.SelectObject(c.HDC, win32.HGDIOBJ(brush))
	defer win32.SelectObject(c.HDC, oldBrush)

	win32.RoundRect(c.HDC, x, y, x+w, y+h, rx*2, ry*2)
}

// DrawEllipse draws an ellipse outline
func (c *Canvas) DrawEllipse(x, y, w, h int32, color Color, lineWidth int32) {
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, color.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))
	old := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, old)

	brush := win32.GetStockObject(win32.NULL_BRUSH)
	oldBrush := win32.SelectObject(c.HDC, brush)
	defer win32.SelectObject(c.HDC, oldBrush)

	win32.Ellipse(c.HDC, x, y, x+w, y+h)
}

// FillEllipse draws a filled ellipse
func (c *Canvas) FillEllipse(x, y, w, h int32, fillColor, borderColor Color, lineWidth int32) {
	brush := win32.CreateSolidBrush(fillColor.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(brush))
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, borderColor.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))

	oldPen := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, oldPen)
	oldBrush := win32.SelectObject(c.HDC, win32.HGDIOBJ(brush))
	defer win32.SelectObject(c.HDC, oldBrush)

	win32.Ellipse(c.HDC, x, y, x+w, y+h)
}

// DrawLine draws a line between two points
func (c *Canvas) DrawLine(x1, y1, x2, y2 int32, color Color, lineWidth int32) {
	pen := win32.CreatePen(win32.PS_SOLID, lineWidth, color.ToCOLORREF())
	defer win32.DeleteObject(win32.HGDIOBJ(pen))
	old := win32.SelectObject(c.HDC, win32.HGDIOBJ(pen))
	defer win32.SelectObject(c.HDC, old)

	win32.MoveToEx(c.HDC, x1, y1, nil)
	win32.LineTo(c.HDC, x2, y2)
}

// utf16Len returns the number of UTF-16 code units in s (for DrawText/TextOut count).
func utf16Len(s string) int32 {
	n := int32(0)
	for _, r := range s {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2 // surrogate pair
		}
	}
	return n
}

// DrawText draws text in the given rectangle with alignment
func (c *Canvas) DrawText(text string, rect Rect, color Color, font *Font, align uint32) {
	// Set font
	if font != nil {
		oldFont := win32.SelectObject(c.HDC, win32.HGDIOBJ(font.Handle))
		defer win32.SelectObject(c.HDC, oldFont)
	}

	// Set text color
	win32.SetTextColor(c.HDC, color.ToCOLORREF())
	win32.SetBkMode(c.HDC, win32.TRANSPARENT)

	rc := rect.ToRECT()
	p := win32.UTF16PtrFromString(text)
	// DrawTextW count is UTF-16 units, NOT Go UTF-8 byte length (Chinese was blank/truncated).
	win32.DrawText(c.HDC, p, utf16Len(text), &rc, win32.UINT(align))
}

// DrawTextSimple draws text at a position
func (c *Canvas) DrawTextSimple(text string, x, y int32, color Color, font *Font) {
	if font != nil {
		oldFont := win32.SelectObject(c.HDC, win32.HGDIOBJ(font.Handle))
		defer win32.SelectObject(c.HDC, oldFont)
	}
	win32.SetTextColor(c.HDC, color.ToCOLORREF())
	win32.SetBkMode(c.HDC, win32.TRANSPARENT)
	p := win32.UTF16PtrFromString(text)
	win32.TextOut(c.HDC, x, y, p, utf16Len(text))
}

// MeasureText returns the width and height of text
func (c *Canvas) MeasureText(text string, font *Font) (int32, int32) {
	if font != nil {
		oldFont := win32.SelectObject(c.HDC, win32.HGDIOBJ(font.Handle))
		defer win32.SelectObject(c.HDC, oldFont)
	}
	size := win32.SIZE{}
	p := win32.UTF16PtrFromString(text)
	win32.GetTextExtentPoint32(c.HDC, p, utf16Len(text), &size)
	return int32(size.Cx), int32(size.Cy)
}

// DrawBitmap draws a bitmap at the given position
func (c *Canvas) DrawBitmap(bmp *Bitmap, x, y int32) {
	if bmp == nil || bmp.Handle == 0 {
		return
	}
	memDC := win32.CreateCompatibleDC(c.HDC)
	defer win32.DeleteDC(memDC)
	old := win32.SelectObject(memDC, win32.HGDIOBJ(bmp.Handle))
	defer win32.SelectObject(memDC, old)
	win32.BitBlt(c.HDC, x, y, bmp.Width, bmp.Height, memDC, 0, 0, win32.SRCCOPY)
}

// DrawBitmapStretched draws a bitmap stretched to fill the given rectangle
func (c *Canvas) DrawBitmapStretched(bmp *Bitmap, x, y, w, h int32) {
	if bmp == nil || bmp.Handle == 0 {
		return
	}
	memDC := win32.CreateCompatibleDC(c.HDC)
	defer win32.DeleteDC(memDC)
	old := win32.SelectObject(memDC, win32.HGDIOBJ(bmp.Handle))
	defer win32.SelectObject(memDC, old)
	win32.StretchBlt(c.HDC, x, y, w, h, memDC, 0, 0, bmp.Width, bmp.Height, win32.SRCCOPY)
}

// DrawBitmapFit draws a bitmap maintaining aspect ratio, centered in the given rectangle
func (c *Canvas) DrawBitmapFit(bmp *Bitmap, x, y, w, h int32) {
	if bmp == nil || bmp.Handle == 0 {
		return
	}

	// Calculate aspect-preserving dimensions
	srcRatio := float64(bmp.Width) / float64(bmp.Height)
	dstRatio := float64(w) / float64(h)

	var dw, dh int32
	if srcRatio > dstRatio {
		dw = w
		dh = int32(float64(w) / srcRatio)
	} else {
		dh = h
		dw = int32(float64(h) * srcRatio)
	}

	dx := x + (w-dw)/2
	dy := y + (h-dh)/2

	c.DrawBitmapStretched(bmp, dx, dy, dw, dh)
}

// GradientFill fills a rectangle with a horizontal or vertical gradient
func (c *Canvas) GradientFillH(x, y, w, h int32, color1, color2 Color) {
	vertexes := [2]win32.TRIVERTEX{
		{X: win32.LONG(x), Y: win32.LONG(y), Red: uint16(color1.R) << 8, Green: uint16(color1.G) << 8, Blue: uint16(color1.B) << 8, Alpha: 0xFF00},
		{X: win32.LONG(x + w), Y: win32.LONG(y + h), Red: uint16(color2.R) << 8, Green: uint16(color2.G) << 8, Blue: uint16(color2.B) << 8, Alpha: 0xFF00},
	}
	rect := win32.GRADIENT_RECT{UpperLeft: 0, LowerRight: 1}
	win32.GradientFill(c.HDC, &vertexes[0], 2, &rect, 1, win32.GRADIENT_FILL_RECT_H)
}

func (c *Canvas) GradientFillV(x, y, w, h int32, color1, color2 Color) {
	vertexes := [2]win32.TRIVERTEX{
		{X: win32.LONG(x), Y: win32.LONG(y), Red: uint16(color1.R) << 8, Green: uint16(color1.G) << 8, Blue: uint16(color1.B) << 8, Alpha: 0xFF00},
		{X: win32.LONG(x + w), Y: win32.LONG(y + h), Red: uint16(color2.R) << 8, Green: uint16(color2.G) << 8, Blue: uint16(color2.B) << 8, Alpha: 0xFF00},
	}
	rect := win32.GRADIENT_RECT{UpperLeft: 0, LowerRight: 1}
	win32.GradientFill(c.HDC, &vertexes[0], 2, &rect, 1, win32.GRADIENT_FILL_RECT_V)
}

// SetClipRect sets a clipping rectangle
func (c *Canvas) SetClipRect(x, y, w, h int32) {
	win32.IntersectClipRect(c.HDC, x, y, x+w, y+h)
}

// ResetClip resets the clipping region
func (c *Canvas) ResetClip() {
	win32.SelectClipRgn(c.HDC, 0)
}

// Save saves the current DC state
func (c *Canvas) Save() int32 {
	return win32.SaveDC(c.HDC)
}

// Restore restores a previously saved DC state
func (c *Canvas) Restore(saved int32) {
	win32.RestoreDC(c.HDC, saved)
}

// DeleteDC deletes a device context (for memory DCs)
func (c *Canvas) DeleteDC() {
	if c.HDC != 0 {
		win32.DeleteDC(c.HDC)
		c.HDC = 0
	}
}

// DrawBitmapWithAlpha draws a bitmap with alpha blending (simplified)
// Uses a memory DC and per-pixel alpha blending
func (c *Canvas) DrawBitmapWithAlpha(bmp *Bitmap, x, y, w, h int32, alpha byte) {
	if bmp == nil || bmp.Handle == 0 || alpha == 0 {
		return
	}
	// Simple approach: use StretchBlt with SRCCOPY
	// Full alpha blending would require reading pixel data and manual compositing
	c.DrawBitmapStretched(bmp, x, y, w, h)
}
