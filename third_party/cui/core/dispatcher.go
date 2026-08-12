package core

import (
	"cui/win32"
	"sync"
	"syscall"
	"unsafe"
)

// wndProcCallback is the syscall-compatible window procedure
var wndProcCallback uintptr

func init() {
	wndProcCallback = syscall.NewCallback(wndProc)
}

// wndProc is the main window procedure that dispatches messages to widgets
func wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	h := win32.HWND(hwnd)

	switch msg {
	case win32.WM_CREATE:
		// Extract the widget pointer from CREATESTRUCT
		cs := (*win32.CREATESTRUCT)(unsafe.Pointer(lparam))
		if cs != nil {
			ptr := uintptr(unsafe.Pointer(cs.CreateParams))
			if ptr != 0 {
				win32.SetWindowLongPtr(h, win32.GWLP_USERDATA, ptr)
			}
		}
		return 0

	case win32.WM_NCCREATE:
		// Enable per-monitor DPI scaling for child windows
		win32.EnableNonClientDpiScaling(h)
		return uintptr(win32.TRUE)
	}

	// Try to get the widget from our registry first
	w := getWidget(h)
	if w != nil {
		// Get the BaseWidget to access its WndProc
		if bw, ok := w.(*BaseWidget); ok {
			return bw.WndProc(msg, wparam, lparam)
		}
	}

	// Check for window-specific handlers
	if wh, ok := windowHandlers.Load(h); ok {
		handler := wh.(*windowHandler)
		if handler.proc != nil {
			return handler.proc(h, msg, wparam, lparam)
		}
	}

	return uintptr(win32.DefWindowProc(h, win32.UINT(msg), win32.WPARAM(wparam), win32.LPARAM(lparam)))
}

// windowHandler stores the window-specific message handler
type windowHandler struct {
	proc func(hwnd win32.HWND, msg uint32, wparam, lparam uintptr) uintptr
}

// Window handler registry
var windowHandlers = &syncMap2{}

type syncMap2 struct {
	m sync.Map
}

func (m *syncMap2) Load(key interface{}) (interface{}, bool) {
	return m.m.Load(key)
}

func (m *syncMap2) Store(key, value interface{}) {
	m.m.Store(key, value)
}

func (m *syncMap2) Delete(key interface{}) {
	m.m.Delete(key)
}

// registerWindowProc registers a window procedure for a top-level window
func registerWindowProc(hwnd win32.HWND, proc func(win32.HWND, uint32, uintptr, uintptr) uintptr) {
	windowHandlers.Store(hwnd, &windowHandler{proc: proc})
}

// unregisterWindowProc unregisters a window procedure
func unregisterWindowProc(hwnd win32.HWND) {
	windowHandlers.Delete(hwnd)
}

// Get the window procedure for a window (used by subclasses)
func getWindowProc(hwnd win32.HWND) func(win32.HWND, uint32, uintptr, uintptr) uintptr {
	if wh, ok := windowHandlers.Load(hwnd); ok {
		return wh.(*windowHandler).proc
	}
	return nil
}

// SubclassEditProc wraps a native Edit control's WndProc to intercept messages
func SubclassEditProc(hwnd win32.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	// Get the original WndProc
	origProc := win32.GetWindowLongPtr(hwnd, win32.GWLP_USERDATA+8) // store original in offset
	if origProc == 0 {
		return uintptr(win32.DefWindowProc(hwnd, win32.UINT(msg), win32.WPARAM(wparam), win32.LPARAM(lparam)))
	}

	// Handle messages we want to intercept
	switch msg {
	case win32.WM_CHAR:
		// Let the widget handle it
		w := getWidget(hwnd)
		if w != nil {
			if bw, ok := w.(*BaseWidget); ok && bw.onChar != nil {
				ke := KeyEvent{Char: rune(wparam), Key: uint32(wparam)}
				bw.onChar(ke)
			}
		}
	case win32.WM_KEYDOWN:
		w := getWidget(hwnd)
		if w != nil {
			if bw, ok := w.(*BaseWidget); ok && bw.onKeyDown != nil {
				ke := KeyEvent{Key: uint32(wparam)}
				bw.onKeyDown(ke)
			}
		}
	}

	// Call the original WndProc
	ret, _, _ := syscall.SyscallN(origProc, uintptr(hwnd), uintptr(msg), wparam, lparam)
	return ret
}
