package core

import "image/color"

// Color represents an RGBA color
type Color struct {
	R, G, B, A uint8
}

// NewColor creates a new Color
func NewColor(r, g, b, a uint8) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// NewRGB creates an opaque Color from RGB values
func NewRGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, A: 255}
}

// ToCOLORREF converts to Win32 COLORREF (0x00BBGGRR)
func (c Color) ToCOLORREF() uint32 {
	return uint32(c.R) | (uint32(c.G) << 8) | (uint32(c.B) << 16)
}

// ToRGBA converts to Go color.RGBA
func (c Color) ToRGBA() color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// FromCOLORREF creates a Color from a Win32 COLORREF
func FromCOLORREF(cr uint32) Color {
	return Color{
		R: byte(cr & 0xFF),
		G: byte((cr >> 8) & 0xFF),
		B: byte((cr >> 16) & 0xFF),
		A: 255,
	}
}

// Predefined colors
var (
	ColorWhite       = NewRGB(255, 255, 255)
	ColorBlack       = NewRGB(0, 0, 0)
	ColorRed         = NewRGB(255, 0, 0)
	ColorGreen       = NewRGB(0, 128, 0)
	ColorBlue        = NewRGB(0, 0, 255)
	ColorGray        = NewRGB(128, 128, 128)
	ColorLightGray   = NewRGB(211, 211, 211)
	ColorDarkGray    = NewRGB(64, 64, 64)
	ColorTransparent = NewColor(0, 0, 0, 0)
	ColorHover       = NewRGB(229, 243, 255)
	ColorPressed     = NewRGB(204, 228, 247)
	ColorDisabled    = NewRGB(204, 204, 204)
	ColorDisabledText = NewRGB(109, 109, 109)
	ColorBorder      = NewRGB(173, 173, 173)
	ColorFocused     = NewRGB(0, 120, 215)
	ColorPlaceholder = NewRGB(150, 150, 150)
)
