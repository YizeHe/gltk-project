package win32

import (
	"syscall"
	"unsafe"
)

var (
	procOpenClipboard   = user32.NewProc("OpenClipboard")
	procCloseClipboard  = user32.NewProc("CloseClipboard")
	procEmptyClipboard  = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
)

// OpenClipboard opens the clipboard
func OpenClipboard(hwnd HWND) BOOL {
	ret, _, _ := procOpenClipboard.Call(uintptr(hwnd))
	return BOOL(ret)
}

// CloseClipboard closes the clipboard
func CloseClipboard() BOOL {
	ret, _, _ := procCloseClipboard.Call()
	return BOOL(ret)
}

// EmptyClipboard empties the clipboard
func EmptyClipboard() BOOL {
	ret, _, _ := procEmptyClipboard.Call()
	return BOOL(ret)
}

// GetClipboardData retrieves data from the clipboard
func GetClipboardData(format UINT) HANDLE {
	ret, _, _ := procGetClipboardData.Call(uintptr(format))
	return HANDLE(ret)
}

// SetClipboardData places data on the clipboard
func SetClipboardData(format UINT, hMem HANDLE) HANDLE {
	ret, _, _ := procSetClipboardData.Call(uintptr(format), uintptr(hMem))
	return HANDLE(ret)
}

// IsClipboardFormatAvailable checks if a clipboard format is available
func IsClipboardFormatAvailable(format UINT) BOOL {
	ret, _, _ := procIsClipboardFormatAvailable.Call(uintptr(format))
	return BOOL(ret)
}

// ClipboardGetText retrieves Unicode text from the clipboard
func ClipboardGetText(hwndOwner HWND) (string, bool) {
	if OpenClipboard(hwndOwner) == FALSE {
		return "", false
	}
	defer CloseClipboard()

	if IsClipboardFormatAvailable(CF_UNICODETEXT) == FALSE {
		return "", false
	}

	hMem := GetClipboardData(CF_UNICODETEXT)
	if hMem == 0 {
		return "", false
	}

	p := GlobalLock(HGLOBAL(hMem))
	if p == nil {
		return "", false
	}
	defer GlobalUnlock(HGLOBAL(hMem))

	// Convert UTF-16 to string
	ptr := (*[1 << 20]uint16)(p)
	n := 0
	for ptr[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(ptr[:n]), true
}

// ClipboardSetText places Unicode text on the clipboard
func ClipboardSetText(hwndOwner HWND, text string) bool {
	if OpenClipboard(hwndOwner) == FALSE {
		return false
	}
	defer CloseClipboard()

	if EmptyClipboard() == FALSE {
		return false
	}

	utf16 := UTF16FromString(text)
	size := uintptr(len(utf16) * 2) // UTF-16 = 2 bytes per char

	hMem := GlobalAlloc(GHND, size)
	if hMem == 0 {
		return false
	}

	p := GlobalLock(hMem)
	if p == nil {
		GlobalFree(hMem)
		return false
	}

	// Copy UTF-16 data to global memory
	dest := (*[1 << 20]uint16)(p)
	copy(dest[:len(utf16)], utf16)

	GlobalUnlock(hMem)

	if SetClipboardData(CF_UNICODETEXT, HANDLE(hMem)) == 0 {
		GlobalFree(hMem)
		return false
	}

	return true
}

// GradientFill from msimg32.dll
var (
	msimg32 = syscall.NewLazyDLL("msimg32.dll")
	procGradientFill = msimg32.NewProc("GradientFill")
)

// GradientFill fills rectangle or triangle with gradient
func GradientFill(hdc HDC, pVertex *TRIVERTEX, nVertex ULONG, pMesh *GRADIENT_RECT, nMesh ULONG, mode ULONG) BOOL {
	ret, _, _ := procGradientFill.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(pVertex)),
		uintptr(nVertex),
		uintptr(unsafe.Pointer(pMesh)),
		uintptr(nMesh),
		uintptr(mode),
	)
	return BOOL(ret)
}

// comctl32 InitCommonControlsEx
var (
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
)

// INITCOMMONCONTROLSEX structure
type INITCOMMONCONTROLSEX struct {
	DwSize DWORD
	DwICC  DWORD
}

// ICC constants
const (
	ICC_LISTVIEW_CLASSES   = 0x00000001
	ICC_TREEVIEW_CLASSES   = 0x00000002
	ICC_BAR_CLASSES        = 0x00000004
	ICC_TAB_CLASSES        = 0x00000008
	ICC_UPDOWN_CLASS       = 0x00000010
	ICC_PROGRESS_CLASS     = 0x00000020
	ICC_HOTKEY_CLASS       = 0x00000040
	ICC_ANIMATE_CLASS      = 0x00000080
	ICC_WIN95_CLASSES      = 0x000000FF
	ICC_DATE_CLASSES       = 0x00000100
	ICC_USEREX_CLASSES     = 0x00000200
	ICC_COOL_CLASSES       = 0x00000400
	ICC_INTERNET_CLASSES   = 0x00000800
	ICC_PAGESCROLLER_CLASS = 0x00001000
	ICC_NATIVEFNTCTL_CLASS = 0x00002000
	ICC_STANDARD_CLASSES   = 0x00004000
	ICC_LINK_CLASS         = 0x00008000
)

// InitCommonControlsEx initializes common controls
func InitCommonControlsEx(init *INITCOMMONCONTROLSEX) BOOL {
	ret, _, _ := procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(init)))
	return BOOL(ret)
}
