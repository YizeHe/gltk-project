package core

import (
	"cui/win32"
	"sync"
)

// Font represents a logical font
type Font struct {
	Handle   win32.HFONT
	Name     string
	Size     int32 // point size
	Bold     bool
	Italic   bool
}

var (
	fontCache   = make(map[string]*Font)
	fontCacheMu sync.Mutex
)

// NewFont creates a new font. Size is in points.
func NewFont(name string, size int32) *Font {
	return NewFontEx(name, size, false, false)
}

// NewFontEx creates a new font with style options
func NewFontEx(name string, size int32, bold, italic bool) *Font {
	key := fontKey(name, size, bold, italic)
	fontCacheMu.Lock()
	if f, ok := fontCache[key]; ok {
		fontCacheMu.Unlock()
		return f
	}
	fontCacheMu.Unlock()

	pixelHeight := DPIToPoints(size)
	weight := int32(win32.FW_NORMAL)
	if bold {
		weight = int32(win32.FW_BOLD)
	}

	lf := win32.LOGFONTW{
		LfHeight:         win32.LONG(pixelHeight),
		LfWidth:          0, // let GDI choose
		LfWeight:         win32.LONG(weight),
		LfItalic:         win32.BYTE(boolToByte(italic)),
		LfCharSet:        win32.DEFAULT_CHARSET,
		LfOutPrecision:   win32.OUT_DEFAULT_PRECIS,
		LfClipPrecision:  win32.CLIP_DEFAULT_PRECIS,
		LfQuality:        win32.CLEARTYPE_QUALITY,
		LfPitchAndFamily: win32.DEFAULT_PITCH | win32.FF_DONTCARE,
	}
	u16:=syscallUTF16(name); for i:=0;i<len(u16)&&i<len(lf.LfFaceName);i++{ lf.LfFaceName[i]=win32.WCHAR(u16[i]) }

	hfont := win32.CreateFontIndirect(&lf)
	if hfont == 0 {
		// Fallback to system font
		hfont = win32.HFONT(win32.GetStockObject(win32.DEFAULT_GUI_FONT))
	}

	f := &Font{
		Handle: hfont,
		Name:   name,
		Size:   size,
		Bold:   bold,
		Italic: italic,
	}

	TrackGDI(win32.HGDIOBJ(hfont), 0, "font")

	fontCacheMu.Lock()
	fontCache[key] = f
	fontCacheMu.Unlock()

	return f
}

// DefaultFont returns the default system font
func DefaultFont() *Font {
	return NewFont("Microsoft YaHei UI", 9)
}

// Dispose releases the font resource
func (f *Font) Dispose() {
	if f.Handle != 0 {
		win32.DeleteObject(win32.HGDIOBJ(f.Handle))
		f.Handle = 0
	}
}

func fontKey(name string, size int32, bold, italic bool) string {
	s := name
	if bold {
		s += ":B"
	}
	if italic {
		s += ":I"
	}
	return s
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func syscallUTF16(s string) []uint16 {
	return append(win32.UTF16FromString(s), 0)
}

// GetTextSize returns the width and height of text rendered with the given font
func GetTextSize(hdc win32.HDC, text string, font *Font) (int32, int32) {
	oldFont := win32.SelectObject(hdc, win32.HGDIOBJ(font.Handle))
	defer win32.SelectObject(hdc, oldFont)

	size := win32.SIZE{}
	p := win32.UTF16PtrFromString(text)
	// UTF-16 code unit count (not UTF-8 byte len)
	n := int32(0)
	for _, r := range text {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2
		}
	}
	win32.GetTextExtentPoint32(hdc, p, n, &size)
	return int32(size.Cx), int32(size.Cy)
}
