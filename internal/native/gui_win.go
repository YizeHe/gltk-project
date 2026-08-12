//go:build windows

package native

import (
	"syscall"
	"unsafe"

	"cui/win32"
)

func setParentHWND(child, parent win32.HWND) {
	if child != 0 && parent != 0 {
		win32.SetParent(child, parent)
	}
}

func guiMessageBox(title, msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	proc.Call(0,
		uintptr(unsafe.Pointer(win32.UTF16PtrFromString(msg))),
		uintptr(unsafe.Pointer(win32.UTF16PtrFromString(title))),
		0)
}

// openFileNameW matches Win32 OPENFILENAMEW (including Vista+ tail fields).
// Size must be correct on amd64 or GetOpenFileNameW fails silently.
type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

// guiOpenFileDialog shows GetOpenFileNameW; filter uses \x00 separators, double-null end.
// owner may be 0; when non-zero the dialog is modal to that HWND.
func guiOpenFileDialog(filter string, owner uintptr) string {
	comdlg := syscall.NewLazyDLL("comdlg32.dll")
	proc := comdlg.NewProc("GetOpenFileNameW")
	buf := make([]uint16, 4096)

	// Ensure double-null terminated filter string
	if len(filter) == 0 {
		filter = "All Files\x00*.*\x00"
	}
	if filter[len(filter)-1] != 0 {
		filter += "\x00"
	}
	if !endsWithDoubleNull(filter) {
		filter += "\x00"
	}

	// Prefer proper UTF-16 conversion that preserves embedded NULs
	filtBuf := buildFilterUTF16(filter)
	ofn := openFileNameW{
		lStructSize:  uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:    owner,
		lpstrFilter:  &filtBuf[0],
		nFilterIndex: 1,
		lpstrFile:    &buf[0],
		nMaxFile:     uint32(len(buf)),
		flags:        0x00080000 | 0x00001000 | 0x00000008, // OFN_EXPLORER | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST
	}
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func endsWithDoubleNull(s string) bool {
	return len(s) >= 2 && s[len(s)-1] == 0 && s[len(s)-2] == 0
}

// buildFilterUTF16 converts a filter that may contain embedded \x00 into UTF-16.
// ASCII path is fine for typical "Desc\x00*.ext\x00" patterns; non-ASCII code
// points are encoded as UTF-16 properly via rune conversion for bytes >= 128.
func buildFilterUTF16(filter string) []uint16 {
	// Treat as UTF-8 source: convert each rune, keep embedded NUL as UTF-16 0.
	runes := []rune(filter)
	out := make([]uint16, 0, len(runes)+2)
	for _, r := range runes {
		if r < 0x10000 {
			out = append(out, uint16(r))
		} else {
			// Surrogate pair (unlikely in file filters)
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	if len(out) == 0 || out[len(out)-1] != 0 {
		out = append(out, 0)
	}
	if len(out) < 2 || out[len(out)-2] != 0 {
		out = append(out, 0)
	}
	return out
}
