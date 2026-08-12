package win32

import "unsafe"

// Basic integer types matching Win32
type (
	BOOL      int32
	BYTE      byte
	WORD      uint16
	DWORD     uint32
	LONG      int32
	ULONG     uint32
	LONG_PTR  uintptr
	ULONG_PTR uintptr
	UINT      uint32
	UINT_PTR  uintptr
	WCHAR     uint16
	LPARAM    uintptr
	WPARAM    uintptr
	LRESULT   uintptr
	ATOM      uint16
)

// Handle types
type (
	HWND     uintptr
	HMENU    uintptr
	HICON    uintptr
	HCURSOR  uintptr
	HBRUSH   uintptr
	HFONT    uintptr
	HPEN     uintptr
	HRGN     uintptr
	HBITMAP  uintptr
	HDC      uintptr
	HGDIOBJ  uintptr
	HMODULE  uintptr
	HGLOBAL  uintptr
	HANDLE   uintptr
	HMONITOR uintptr
)

// Pointer types
type (
	LPCWSTR  *uint16
	LPWSTR   *uint16
	LPCVOID  unsafe.Pointer
	LPVOID   unsafe.Pointer
	LPDWORD  *uint32
	LPBYTE   *byte
)

// POINT structure
type POINT struct {
	X LONG
	Y LONG
}

// RECT structure
type RECT struct {
	Left   LONG
	Top    LONG
	Right  LONG
	Bottom LONG
}

// SIZE structure
type SIZE struct {
	Cx LONG
	Cy LONG
}

// MSG structure
type MSG struct {
	Hwnd    HWND
	Message UINT
	WParam  WPARAM
	LParam  LPARAM
	Time    DWORD
	Pt      POINT
}

// WNDCLASSEX structure
type WNDCLASSEX struct {
	CbSize        UINT
	Style         UINT
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     HMODULE
	HIcon         HICON
	HCursor       HCURSOR
	HbrBackground HBRUSH
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       HICON
}

// CREATESTRUCT structure
type CREATESTRUCT struct {
	CreateParams LPCVOID
	Inst         HMENU
	Menu         HMENU
	Parent       HWND
	Cy           int32
	Cx           int32
	Y            int32
	X            int32
	Style        LONG
	Name         *uint16
	Class        *uint16
	ExStyle      DWORD
}

// PAINTSTRUCT structure
type PAINTSTRUCT struct {
	Hdc         HDC
	Erase       BOOL
	RcPaint     RECT
	Restore     BOOL
	IncUpdate   BOOL
	RgbReserved [32]byte
}

// LOGFONTW structure
type LOGFONTW struct {
	LfHeight         LONG
	LfWidth          LONG
	LfEscapement     LONG
	LfOrientation    LONG
	LfWeight         LONG
	LfItalic         BYTE
	LfUnderline      BYTE
	LfStrikeOut      BYTE
	LfCharSet        BYTE
	LfOutPrecision   BYTE
	LfClipPrecision  BYTE
	LfQuality        BYTE
	LfPitchAndFamily BYTE
	LfFaceName       [32]WCHAR
}

// NONCLIENTMETRICSW structure
type NONCLIENTMETRICSW struct {
	CbSize           UINT
	IBorderWidth     int32
	IScrollWidth     int32
	ISmCaptionHeight int32
	LfSmCaptionFont  LOGFONTW
	IMenuHeight      int32
	LfMenuFont       LOGFONTW
	LfStatusFont     LOGFONTW
	LfMessageFont    LOGFONTW
}

// TRACKMOUSEEVENT structure
type TRACKMOUSEEVENT struct {
	CbSize      DWORD
	DwFlags     DWORD
	HwndTrack   HWND
	DwHoverTime DWORD
}

// MENUITEMINFOW structure
type MENUITEMINFOW struct {
	CbSize        UINT
	FMask         UINT
	FType         UINT
	FState        UINT
	WID           UINT
	HSubMenu      HMENU
	HbmpChecked   HBITMAP
	HbmpUnchecked HBITMAP
	DwItemData    ULONG_PTR
	DwTypeData    *uint16
	Cch           UINT
	HbmpItem      HBITMAP
}

// GRADIENT_RECT for GradientFill
type GRADIENT_RECT struct {
	UpperLeft  ULONG
	LowerRight ULONG
}

// TRIVERTEX for GradientFill
type TRIVERTEX struct {
	X     LONG
	Y     LONG
	Red   uint16
	Green uint16
	Blue  uint16
	Alpha uint16
}

// GDIPLUS StartupInput
type GdiplusStartupInput struct {
	GdiplusVersion          uint32
	DebugEventCallback      uintptr
	SuppressBackgroundThread BOOL
	SuppressExternalCodecs  BOOL
}

// GDI+ BitmapData
type GpBitmap struct {
	_ [1]byte // opaque
}

// Helper: convert int to uintptr for unsafe casts
func Ptr(v int) uintptr     { return uintptr(v) }
func PtrU(v uint) uintptr   { return uintptr(v) }
func Uintptr(p unsafe.Pointer) uintptr { return uintptr(p) }
