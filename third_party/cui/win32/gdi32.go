package win32

import (
	"syscall"
	"unsafe"
)

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	procCreateFontIndirectW    = gdi32.NewProc("CreateFontIndirectW")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procCreateRectRgn          = gdi32.NewProc("CreateRectRgn")
	procSelectClipRgn          = gdi32.NewProc("SelectClipRgn")
	procRectangle              = gdi32.NewProc("Rectangle")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procEllipse                = gdi32.NewProc("Ellipse")
	// FillRect / FrameRect / DrawTextW live in user32.dll (see user32.go)
	procTextOutW               = gdi32.NewProc("TextOutW")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procSetBkColor             = gdi32.NewProc("SetBkColor")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procStretchBlt             = gdi32.NewProc("StretchBlt")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procGetObjectW             = gdi32.NewProc("GetObjectW")
	procGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procSetPixel               = gdi32.NewProc("SetPixel")
	procGetPixel               = gdi32.NewProc("GetPixel")
	procCreateBitmap           = gdi32.NewProc("CreateBitmap")
	procCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	procPatBlt                 = gdi32.NewProc("PatBlt")
	procSaveDC                 = gdi32.NewProc("SaveDC")
	procRestoreDC              = gdi32.NewProc("RestoreDC")
	procIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	procExcludeClipRect        = gdi32.NewProc("ExcludeClipRect")
	procGetClipBox             = gdi32.NewProc("GetClipBox")
	procExtCreatePen           = gdi32.NewProc("ExtCreatePen")
	procPolygon                = gdi32.NewProc("Polygon")
	procPolyline               = gdi32.NewProc("Polyline")
	procAngleArc               = gdi32.NewProc("AngleArc")
	procArc                    = gdi32.NewProc("Arc")
	procPie                    = gdi32.NewProc("Pie")
	procChord                  = gdi32.NewProc("Chord")
	procSetArcDirection        = gdi32.NewProc("SetArcDirection")
	procGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	procCreatePatternBrush     = gdi32.NewProc("CreatePatternBrush")
	procCreateBitmapIndirect   = gdi32.NewProc("CreateBitmapIndirect")
	procGetBitmapBits          = gdi32.NewProc("GetBitmapBits")
	procSetBitmapBits          = gdi32.NewProc("SetBitmapBits")
	procGetCurrentObject       = gdi32.NewProc("GetCurrentObject")
	procGetObjectType          = gdi32.NewProc("GetObjectType")
	procCombineRgn             = gdi32.NewProc("CombineRgn")
	procFillRgn                = gdi32.NewProc("FillRgn")
	procFrameRgn               = gdi32.NewProc("FrameRgn")
	procPaintRgn               = gdi32.NewProc("PaintRgn")
	procSetViewportOrgEx       = gdi32.NewProc("SetViewportOrgEx")
	procSetViewportExtEx       = gdi32.NewProc("SetViewportExtEx")
	procSetWindowOrgEx         = gdi32.NewProc("SetWindowOrgEx")
	procSetWindowExtEx         = gdi32.NewProc("SetWindowExtEx")
	procMulDiv                 = syscall.NewLazyDLL("kernel32.dll").NewProc("MulDiv")
)

// CreateFontIndirect creates a logical font
func CreateFontIndirect(lf *LOGFONTW) HFONT {
	ret, _, _ := procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(lf)))
	return HFONT(ret)
}

// CreateFont creates a logical font with specified attributes
func CreateFont(height, width, escapement, orientation, weight int32,
	italic, underline, strikeout, charset, outPrecision, clipPrecision,
	quality, pitchAndFamily byte, faceName *uint16) HFONT {
	ret, _, _ := procCreateFontW.Call(
		uintptr(height), uintptr(width), uintptr(escapement), uintptr(orientation),
		uintptr(weight), uintptr(italic), uintptr(underline), uintptr(strikeout),
		uintptr(charset), uintptr(outPrecision), uintptr(clipPrecision),
		uintptr(quality), uintptr(pitchAndFamily), uintptr(unsafe.Pointer(faceName)),
	)
	return HFONT(ret)
}

// DeleteObject deletes a logical pen, brush, font, bitmap, region, or palette
func DeleteObject(obj HGDIOBJ) BOOL {
	ret, _, _ := procDeleteObject.Call(uintptr(obj))
	return BOOL(ret)
}

// SelectObject selects an object into the specified DC
func SelectObject(hdc HDC, obj HGDIOBJ) HGDIOBJ {
	ret, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(obj))
	return HGDIOBJ(ret)
}

// CreateSolidBrush creates a logical brush with the specified solid color
func CreateSolidBrush(color uint32) HBRUSH {
	ret, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return HBRUSH(ret)
}

// CreatePen creates a logical pen with the specified style, width, and color
func CreatePen(style, width int32, color uint32) HPEN {
	ret, _, _ := procCreatePen.Call(uintptr(style), uintptr(width), uintptr(color))
	return HPEN(ret)
}

// CreateRectRgn creates a rectangular region
func CreateRectRgn(x1, y1, x2, y2 int32) HRGN {
	ret, _, _ := procCreateRectRgn.Call(
		uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2),
	)
	return HRGN(ret)
}

// SelectClipRgn selects a region as the clipping region
func SelectClipRgn(hdc HDC, rgn HRGN) int32 {
	ret, _, _ := procSelectClipRgn.Call(uintptr(hdc), uintptr(rgn))
	return int32(ret)
}

// Rectangle draws a rectangle
func Rectangle(hdc HDC, left, top, right, bottom int32) BOOL {
	ret, _, _ := procRectangle.Call(
		uintptr(hdc), uintptr(left), uintptr(top), uintptr(right), uintptr(bottom),
	)
	return BOOL(ret)
}

// RoundRect draws a rectangle with rounded corners
func RoundRect(hdc HDC, left, top, right, bottom, width, height int32) BOOL {
	ret, _, _ := procRoundRect.Call(
		uintptr(hdc), uintptr(left), uintptr(top),
		uintptr(right), uintptr(bottom), uintptr(width), uintptr(height),
	)
	return BOOL(ret)
}

// Ellipse draws an ellipse
func Ellipse(hdc HDC, left, top, right, bottom int32) BOOL {
	ret, _, _ := procEllipse.Call(
		uintptr(hdc), uintptr(left), uintptr(top), uintptr(right), uintptr(bottom),
	)
	return BOOL(ret)
}

// TextOut writes a character string at the specified location
func TextOut(hdc HDC, x, y int32, text *uint16, count int32) BOOL {
	ret, _, _ := procTextOutW.Call(
		uintptr(hdc), uintptr(x), uintptr(y),
		uintptr(unsafe.Pointer(text)), uintptr(count),
	)
	return BOOL(ret)
}

// SetTextColor sets the text color
func SetTextColor(hdc HDC, color uint32) uint32 {
	ret, _, _ := procSetTextColor.Call(uintptr(hdc), uintptr(color))
	return uint32(ret)
}

// SetBkColor sets the background color
func SetBkColor(hdc HDC, color uint32) uint32 {
	ret, _, _ := procSetBkColor.Call(uintptr(hdc), uintptr(color))
	return uint32(ret)
}

// SetBkMode sets the background mix mode
func SetBkMode(hdc HDC, mode int32) int32 {
	ret, _, _ := procSetBkMode.Call(uintptr(hdc), uintptr(mode))
	return int32(ret)
}

// BitBlt performs a bit-block transfer
func BitBlt(hdc HDC, x, y, cx, cy int32, hdcSrc HDC, x1, y1 int32, rop uint32) BOOL {
	ret, _, _ := procBitBlt.Call(
		uintptr(hdc), uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		uintptr(hdcSrc), uintptr(x1), uintptr(y1), uintptr(rop),
	)
	return BOOL(ret)
}

// StretchBlt performs a bit-block transfer with stretching/compressing
func StretchBlt(hdcDest HDC, xDest, yDest, wDest, hDest int32,
	hdcSrc HDC, xSrc, ySrc, wSrc, hSrc int32, rop uint32) BOOL {
	ret, _, _ := procStretchBlt.Call(
		uintptr(hdcDest), uintptr(xDest), uintptr(yDest), uintptr(wDest), uintptr(hDest),
		uintptr(hdcSrc), uintptr(xSrc), uintptr(ySrc), uintptr(wSrc), uintptr(hSrc),
		uintptr(rop),
	)
	return BOOL(ret)
}

// CreateCompatibleDC creates a memory DC compatible with the specified DC
func CreateCompatibleDC(hdc HDC) HDC {
	ret, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	return HDC(ret)
}

// DeleteDC deletes a device context (memory DCs).
func DeleteDC(hdc HDC) BOOL {
	ret, _, _ := procDeleteDC.Call(uintptr(hdc))
	return BOOL(ret)
}

// CreateCompatibleBitmap creates a bitmap compatible with the specified DC
func CreateCompatibleBitmap(hdc HDC, cx, cy int32) HBITMAP {
	ret, _, _ := procCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(cx), uintptr(cy))
	return HBITMAP(ret)
}

// GetObject retrieves information for the specified graphics object
func GetObject(obj HGDIOBJ, bufSize int32, buf unsafe.Pointer) int32 {
	ret, _, _ := procGetObjectW.Call(uintptr(obj), uintptr(bufSize), uintptr(buf))
	return int32(ret)
}

// GetTextExtentPoint32 computes the width and height of the specified string
func GetTextExtentPoint32(hdc HDC, text *uint16, count int32, size *SIZE) BOOL {
	ret, _, _ := procGetTextExtentPoint32W.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(text)), uintptr(count),
		uintptr(unsafe.Pointer(size)),
	)
	return BOOL(ret)
}

// MoveToEx updates the current position
func MoveToEx(hdc HDC, x, y int32, prevPoint *POINT) BOOL {
	ret, _, _ := procMoveToEx.Call(
		uintptr(hdc), uintptr(x), uintptr(y), uintptr(unsafe.Pointer(prevPoint)),
	)
	return BOOL(ret)
}

// LineTo draws a line from the current position up to, but not including, the specified point
func LineTo(hdc HDC, x, y int32) BOOL {
	ret, _, _ := procLineTo.Call(uintptr(hdc), uintptr(x), uintptr(y))
	return BOOL(ret)
}

// GetStockObject retrieves a stock pen, brush, or font
func GetStockObject(obj int32) HGDIOBJ {
	ret, _, _ := procGetStockObject.Call(uintptr(obj))
	return HGDIOBJ(ret)
}

// SetPixel sets the pixel at the specified coordinates to the specified color
func SetPixel(hdc HDC, x, y int32, color uint32) uint32 {
	ret, _, _ := procSetPixel.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(color))
	return uint32(ret)
}

// GetPixel retrieves the color value of the pixel at the specified coordinates
func GetPixel(hdc HDC, x, y int32) uint32 {
	ret, _, _ := procGetPixel.Call(uintptr(hdc), uintptr(x), uintptr(y))
	return uint32(ret)
}

// CreateDIBSection creates a DIB that applications can write to directly
func CreateDIBSection(hdc HDC, bmi *BITMAPINFO, usage uint32, bits *unsafe.Pointer, section HANDLE, offset DWORD) HBITMAP {
	ret, _, _ := procCreateDIBSection.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(bmi)), uintptr(usage),
		uintptr(unsafe.Pointer(bits)), uintptr(section), uintptr(offset),
	)
	return HBITMAP(ret)
}

// PatBlt paints the specified rectangle using the brush currently selected
func PatBlt(hdc HDC, x, y, w, h int32, rop uint32) BOOL {
	ret, _, _ := procPatBlt.Call(
		uintptr(hdc), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(rop),
	)
	return BOOL(ret)
}

// SaveDC saves the current state of the specified DC
func SaveDC(hdc HDC) int32 {
	ret, _, _ := procSaveDC.Call(uintptr(hdc))
	return int32(ret)
}

// RestoreDC restores a device context to the specified state
func RestoreDC(hdc HDC, savedDC int32) BOOL {
	ret, _, _ := procRestoreDC.Call(uintptr(hdc), uintptr(savedDC))
	return BOOL(ret)
}

// IntersectClipRect creates a new clipping region from the intersection
func IntersectClipRect(hdc HDC, left, top, right, bottom int32) int32 {
	ret, _, _ := procIntersectClipRect.Call(
		uintptr(hdc), uintptr(left), uintptr(top), uintptr(right), uintptr(bottom),
	)
	return int32(ret)
}

// ExcludeClipRect creates a new clipping region from the existing one minus the specified rectangle
func ExcludeClipRect(hdc HDC, left, top, right, bottom int32) int32 {
	ret, _, _ := procExcludeClipRect.Call(
		uintptr(hdc), uintptr(left), uintptr(top), uintptr(right), uintptr(bottom),
	)
	return int32(ret)
}

// GetClipBox retrieves the dimensions of the tightest bounding rectangle
func GetClipBox(hdc HDC, rect *RECT) int32 {
	ret, _, _ := procGetClipBox.Call(uintptr(hdc), uintptr(unsafe.Pointer(rect)))
	return int32(ret)
}

// ExtCreatePen creates a logical cosmetic or geometric pen
func ExtCreatePen(style, width uint32, lb *LOGBRUSH, styleCount DWORD, styleData *DWORD) HPEN {
	ret, _, _ := procExtCreatePen.Call(
		uintptr(style), uintptr(width), uintptr(unsafe.Pointer(lb)),
		uintptr(styleCount), uintptr(unsafe.Pointer(styleData)),
	)
	return HPEN(ret)
}

// Polygon draws a polygon
func Polygon(hdc HDC, pts *POINT, count int32) BOOL {
	ret, _, _ := procPolygon.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(pts)), uintptr(count),
	)
	return BOOL(ret)
}

// Polyline draws a series of line segments
func Polyline(hdc HDC, pts *POINT, count int32) BOOL {
	ret, _, _ := procPolyline.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(pts)), uintptr(count),
	)
	return BOOL(ret)
}

// GetDeviceCaps retrieves device-specific information
func GetDeviceCaps(hdc HDC, index int32) int32 {
	ret, _, _ := procGetDeviceCaps.Call(uintptr(hdc), uintptr(index))
	return int32(ret)
}

// CombineRgn combines two regions
func CombineRgn(dest, src1, src2 HRGN, mode int32) int32 {
	ret, _, _ := procCombineRgn.Call(
		uintptr(dest), uintptr(src1), uintptr(src2), uintptr(mode),
	)
	return int32(ret)
}

// FillRgn fills the specified region using the specified brush
func FillRgn(hdc HDC, rgn HRGN, brush HBRUSH) BOOL {
	ret, _, _ := procFillRgn.Call(uintptr(hdc), uintptr(rgn), uintptr(brush))
	return BOOL(ret)
}

// FrameRgn draws a border around the specified region
func FrameRgn(hdc HDC, rgn HRGN, brush HBRUSH, w, h int32) BOOL {
	ret, _, _ := procFrameRgn.Call(
		uintptr(hdc), uintptr(rgn), uintptr(brush), uintptr(w), uintptr(h),
	)
	return BOOL(ret)
}

// PaintRgn paints the specified region using the brush currently selected
func PaintRgn(hdc HDC, rgn HRGN) BOOL {
	ret, _, _ := procPaintRgn.Call(uintptr(hdc), uintptr(rgn))
	return BOOL(ret)
}

// GetObjectType returns the type of the specified object
func GetObjectType(obj HGDIOBJ) DWORD {
	ret, _, _ := procGetObjectType.Call(uintptr(obj))
	return DWORD(ret)
}

// GetCurrentObject returns the currently selected object of the specified type
func GetCurrentObject(hdc HDC, objType UINT) HGDIOBJ {
	ret, _, _ := procGetCurrentObject.Call(uintptr(hdc), uintptr(objType))
	return HGDIOBJ(ret)
}

// MulDiv multiplies two 32-bit values and then divides the 64-bit result by a third 32-bit value
func MulDiv(number, numerator, denominator int32) int32 {
	ret, _, _ := procMulDiv.Call(
		uintptr(number), uintptr(numerator), uintptr(denominator),
	)
	return int32(ret)
}

// BITMAPINFO structures
type RGBQUAD struct {
	RgbBlue     byte
	RgbGreen    byte
	RgbRed      byte
	RgbReserved byte
}

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]RGBQUAD
}

// LOGBRUSH structure
type LOGBRUSH struct {
	LbStyle UINT
	LbColor uint32
	LbHatch uintptr
}

// Device caps indices
const (
	HORZRES      = 8
	VERTRES      = 10
	BITSPIXEL    = 12
	PLANES       = 14
	LOGPIXELSX   = 88
	LOGPIXELSY   = 90
	PHYSICALWIDTH   = 110
	PHYSICALHEIGHT  = 111
	PHYSICALOFFSETX = 112
	PHYSICALOFFSETY = 113
)

// CombineRgn modes
const (
	RGN_AND  = 1
	RGN_OR   = 2
	RGN_XOR  = 3
	RGN_DIFF = 4
	RGN_COPY = 5
)

// GDI object types
const (
	OBJ_PEN          = 1
	OBJ_BRUSH        = 2
	OBJ_DC           = 3
	OBJ_METADC       = 4
	OBJ_PAL          = 5
	OBJ_FONT         = 6
	OBJ_BITMAP       = 7
	OBJ_REGION       = 8
	OBJ_METAFILE     = 9
	OBJ_MEMDC        = 10
	OBJ_EXTPEN       = 11
	OBJ_ENHMETADC    = 12
	OBJ_ENHMETAFILE  = 13
	OBJ_COLORSPACE   = 14
)

// Arc directions
const (
	AD_COUNTERCLOCKWISE = 1
	AD_CLOCKWISE        = 2
)
