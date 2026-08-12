package win32

import (
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	// Window management
	procRegisterClassExW          = user32.NewProc("RegisterClassExW")
	procUnregisterClassW          = user32.NewProc("UnregisterClassW")
	procCreateWindowExW           = user32.NewProc("CreateWindowExW")
	procDestroyWindow             = user32.NewProc("DestroyWindow")
	procShowWindow                = user32.NewProc("ShowWindow")
	procUpdateWindow              = user32.NewProc("UpdateWindow")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
	procMoveWindow                = user32.NewProc("MoveWindow")
	procGetClientRect             = user32.NewProc("GetClientRect")
	procGetWindowRect             = user32.NewProc("GetWindowRect")
	procInvalidateRect            = user32.NewProc("InvalidateRect")
	procRedrawWindow              = user32.NewProc("RedrawWindow")
	procBeginPaint                = user32.NewProc("BeginPaint")
	procEndPaint                  = user32.NewProc("EndPaint")
	procDefWindowProcW            = user32.NewProc("DefWindowProcW")
	procGetMessageW               = user32.NewProc("GetMessageW")
	procTranslateMessage          = user32.NewProc("TranslateMessage")
	procDispatchMessageW          = user32.NewProc("DispatchMessageW")
	procPostQuitMessage           = user32.NewProc("PostQuitMessage")
	procPostThreadMessageW        = user32.NewProc("PostThreadMessageW")
	procSendMessageW              = user32.NewProc("SendMessageW")
	procPostMessageW              = user32.NewProc("PostMessageW")
	procSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowTextW            = user32.NewProc("SetWindowTextW")
	procGetWindowTextW            = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW      = user32.NewProc("GetWindowTextLengthW")
	procLoadCursorW               = user32.NewProc("LoadCursorW")
	procSetCursor                 = user32.NewProc("SetCursor")
	procGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	procSetTimer                  = user32.NewProc("SetTimer")
	procKillTimer                 = user32.NewProc("KillTimer")
	procTrackMouseEvent           = user32.NewProc("TrackMouseEvent")
	procGetDC                     = user32.NewProc("GetDC")
	procReleaseDC                 = user32.NewProc("ReleaseDC")
	procGetWindowDC               = user32.NewProc("GetWindowDC")
	procEnableWindow              = user32.NewProc("EnableWindow")
	procIsWindowEnabled           = user32.NewProc("IsWindowEnabled")
	procIsWindowVisible           = user32.NewProc("IsWindowVisible")
	procSetFocus                  = user32.NewProc("SetFocus")
	procGetFocus                  = user32.NewProc("GetFocus")
	procSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow       = user32.NewProc("GetForegroundWindow")
	procSetActiveWindow           = user32.NewProc("SetActiveWindow")
	procSetCapture                = user32.NewProc("SetCapture")
	procReleaseCapture            = user32.NewProc("ReleaseCapture")
	procClientToScreen            = user32.NewProc("ClientToScreen")
	procScreenToClient            = user32.NewProc("ScreenToClient")
	procGetCursorPos              = user32.NewProc("GetCursorPos")
	procMapWindowPoints           = user32.NewProc("MapWindowPoints")
	procGetParent                 = user32.NewProc("GetParent")
	procSetParent                 = user32.NewProc("SetParent")
	procIsWindow                  = user32.NewProc("IsWindow")
	procMessageBoxW               = user32.NewProc("MessageBoxW")
	procSetWindowLongW            = user32.NewProc("SetWindowLongW")
	procGetWindowLongW            = user32.NewProc("GetWindowLongW")
	procAdjustWindowRectEx        = user32.NewProc("AdjustWindowRectEx")
	procGetAsyncKeyState          = user32.NewProc("GetAsyncKeyState")
	procGetKeyState               = user32.NewProc("GetKeyState")
	procPeekMessageW              = user32.NewProc("PeekMessageW")
	procWaitMessage               = user32.NewProc("WaitMessage")
	procTranslateAcceleratorW     = user32.NewProc("TranslateAcceleratorW")
	procSetWindowLongPtr          = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtr          = user32.NewProc("GetWindowLongPtrW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procRegisterWindowMessageW    = user32.NewProc("RegisterWindowMessageW")
	procSendMessageTimeoutW       = user32.NewProc("SendMessageTimeoutW")
	procIsWindowUnicode           = user32.NewProc("IsWindowUnicode")
	procGetWindow                 = user32.NewProc("GetWindow")
	// GDI helpers that actually live in user32
	procFillRect                  = user32.NewProc("FillRect")
	procFrameRect                 = user32.NewProc("FrameRect")
	procDrawTextW                 = user32.NewProc("DrawTextW")

	// DPI
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem              = user32.NewProc("GetDpiForSystem")
	procEnableNonClientDpiScaling    = user32.NewProc("EnableNonClientDpiScaling")
	procSetProcessDPIAware           = user32.NewProc("SetProcessDPIAware")

	// Menu
	procCreateMenu             = user32.NewProc("CreateMenu")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procInsertMenuW            = user32.NewProc("InsertMenuW")
	procInsertMenuItemW        = user32.NewProc("InsertMenuItemW")
	procSetMenu                = user32.NewProc("SetMenu")
	procGetMenu                = user32.NewProc("GetMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procEnableMenuItem         = user32.NewProc("EnableMenuItem")
	procCheckMenuItem          = user32.NewProc("CheckMenuItem")
	procGetMenuItemCount       = user32.NewProc("GetMenuItemCount")
	procGetMenuItemID          = user32.NewProc("GetMenuItemID")
	procGetSubMenu             = user32.NewProc("GetSubMenu")
	procSetMenuItemInfoW       = user32.NewProc("SetMenuItemInfoW")
	procGetMenuItemInfoW       = user32.NewProc("GetMenuItemInfoW")
	procRemoveMenu             = user32.NewProc("RemoveMenu")
	procDeleteMenu             = user32.NewProc("DeleteMenu")
	procEndMenu                = user32.NewProc("EndMenu")
	procModifyMenuW            = user32.NewProc("ModifyMenuW")
	procHiliteMenuItem         = user32.NewProc("HiliteMenuItem")
	procGetMenuState           = user32.NewProc("GetMenuState")
	procGetMenuStringW         = user32.NewProc("GetMenuStringW")

	// Message box / misc
	procMessageBeep             = user32.NewProc("MessageBeep")
	procFlashWindow             = user32.NewProc("FlashWindow")
	procGetMessagePos           = user32.NewProc("GetMessagePos")
	procGetMessageTime          = user32.NewProc("GetMessageTime")
)

// Unicode helpers
func UTF16PtrFromString(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func UTF16FromString(s string) []uint16 {
	p, _ := syscall.UTF16FromString(s)
	return p
}

// RegisterClassEx registers a window class
func RegisterClassEx(wc *WNDCLASSEX) ATOM {
	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	return ATOM(ret)
}

// UnregisterClass unregisters a window class
func UnregisterClass(className *uint16, hInst HMODULE) BOOL {
	ret, _, _ := procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), uintptr(hInst))
	return BOOL(ret)
}

// CreateWindowEx creates a window
func CreateWindowEx(exStyle DWORD, className, windowName *uint16,
	style DWORD, x, y, w, h int32,
	parent HWND, menu HMENU, hInst HMODULE, lpParam unsafe.Pointer) HWND {
	ret, _, _ := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(menu), uintptr(hInst),
		uintptr(lpParam),
	)
	return HWND(ret)
}

// DestroyWindow destroys a window
func DestroyWindow(hwnd HWND) BOOL {
	ret, _, _ := procDestroyWindow.Call(uintptr(hwnd))
	return BOOL(ret)
}

// ShowWindow shows or hides a window
func ShowWindow(hwnd HWND, cmdShow int32) BOOL {
	ret, _, _ := procShowWindow.Call(uintptr(hwnd), uintptr(cmdShow))
	return BOOL(ret)
}

// UpdateWindow updates the client area
func UpdateWindow(hwnd HWND) BOOL {
	ret, _, _ := procUpdateWindow.Call(uintptr(hwnd))
	return BOOL(ret)
}

// SetWindowPos changes the size, position, and Z order
func SetWindowPos(hwnd, hwndInsertAfter HWND, x, y, cx, cy int32, flags UINT) BOOL {
	ret, _, _ := procSetWindowPos.Call(
		uintptr(hwnd), uintptr(hwndInsertAfter),
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		uintptr(flags),
	)
	return BOOL(ret)
}

// MoveWindow changes the position and dimensions
func MoveWindow(hwnd HWND, x, y, w, h int32, repaint BOOL) BOOL {
	ret, _, _ := procMoveWindow.Call(
		uintptr(hwnd), uintptr(x), uintptr(y),
		uintptr(w), uintptr(h), uintptr(repaint),
	)
	return BOOL(ret)
}

// GetClientRect retrieves the client area rectangle
func GetClientRect(hwnd HWND, rect *RECT) BOOL {
	ret, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(rect)))
	return BOOL(ret)
}

// GetWindowRect retrieves the window rectangle
func GetWindowRect(hwnd HWND, rect *RECT) BOOL {
	ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(rect)))
	return BOOL(ret)
}

// InvalidateRect invalidates a client area rectangle
func InvalidateRect(hwnd HWND, rect *RECT, erase BOOL) BOOL {
	ret, _, _ := procInvalidateRect.Call(
		uintptr(hwnd), uintptr(unsafe.Pointer(rect)), uintptr(erase),
	)
	return BOOL(ret)
}

// RedrawWindow updates the specified rectangle or region
func RedrawWindow(hwnd HWND, rcUpdate *RECT, hrgnUpdate HRGN, flags UINT) BOOL {
	ret, _, _ := procRedrawWindow.Call(
		uintptr(hwnd), uintptr(unsafe.Pointer(rcUpdate)),
		uintptr(hrgnUpdate), uintptr(flags),
	)
	return BOOL(ret)
}

// BeginPaint prepares a window for painting
func BeginPaint(hwnd HWND, ps *PAINTSTRUCT) HDC {
	ret, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ps)))
	return HDC(ret)
}

// EndPaint marks the end of painting
func EndPaint(hwnd HWND, ps *PAINTSTRUCT) BOOL {
	ret, _, _ := procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ps)))
	return BOOL(ret)
}

// DefWindowProc provides default message processing
func DefWindowProc(hwnd HWND, msg UINT, wParam WPARAM, lParam LPARAM) LRESULT {
	ret, _, _ := procDefWindowProcW.Call(
		uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam),
	)
	return LRESULT(ret)
}

// GetMessage gets a message from the message queue
func GetMessage(msg *MSG, hwnd HWND, msgFilterMin, msgFilterMax UINT) BOOL {
	ret, _, _ := procGetMessageW.Call(
		uintptr(unsafe.Pointer(msg)), uintptr(hwnd),
		uintptr(msgFilterMin), uintptr(msgFilterMax),
	)
	return BOOL(ret)
}

// TranslateMessage translates virtual-key messages
func TranslateMessage(msg *MSG) BOOL {
	ret, _, _ := procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
	return BOOL(ret)
}

// DispatchMessage dispatches a message to a window procedure
func DispatchMessage(msg *MSG) LRESULT {
	ret, _, _ := procDispatchMessageW.Call(uintptr(unsafe.Pointer(msg)))
	return LRESULT(ret)
}

// PostQuitMessage posts a quit message to the *calling* thread's queue.
// Prefer App.Quit / PostThreadMessage when quitting from another thread.
func PostQuitMessage(exitCode int32) {
	procPostQuitMessage.Call(uintptr(exitCode))
}

// PostThreadMessage posts a message to a thread's message queue.
func PostThreadMessage(threadID uint32, msg UINT, wParam WPARAM, lParam LPARAM) BOOL {
	ret, _, _ := procPostThreadMessageW.Call(
		uintptr(threadID), uintptr(msg), uintptr(wParam), uintptr(lParam),
	)
	return BOOL(ret)
}

// SendMessage sends a message to a window
func SendMessage(hwnd HWND, msg UINT, wParam WPARAM, lParam LPARAM) LRESULT {
	ret, _, _ := procSendMessageW.Call(
		uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam),
	)
	return LRESULT(ret)
}

// PostMessage posts a message to a window's message queue
func PostMessage(hwnd HWND, msg UINT, wParam WPARAM, lParam LPARAM) BOOL {
	ret, _, _ := procPostMessageW.Call(
		uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam),
	)
	return BOOL(ret)
}

// SetWindowLongPtr sets a window attribute
func SetWindowLongPtr(hwnd HWND, index int32, value uintptr) uintptr {
	ret, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), value)
	return ret
}

// GetWindowLongPtr gets a window attribute
func GetWindowLongPtr(hwnd HWND, index int32) uintptr {
	ret, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return ret
}

// SetWindowLong sets a window attribute (32-bit compatible)
func SetWindowLong(hwnd HWND, index int32, value int32) int32 {
	ret, _, _ := procSetWindowLongW.Call(uintptr(hwnd), uintptr(index), uintptr(value))
	return int32(ret)
}

// GetWindowLong gets a window attribute (32-bit compatible)
func GetWindowLong(hwnd HWND, index int32) int32 {
	ret, _, _ := procGetWindowLongW.Call(uintptr(hwnd), uintptr(index))
	return int32(ret)
}

// SetWindowText sets the window text
func SetWindowText(hwnd HWND, text *uint16) BOOL {
	ret, _, _ := procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(text)))
	return BOOL(ret)
}

// GetWindowText gets the window text
func GetWindowText(hwnd HWND, buf *uint16, maxCount int32) int32 {
	ret, _, _ := procGetWindowTextW.Call(
		uintptr(hwnd), uintptr(unsafe.Pointer(buf)), uintptr(maxCount),
	)
	return int32(ret)
}

// GetWindowTextLength gets the length of the window text
func GetWindowTextLength(hwnd HWND) int32 {
	ret, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	return int32(ret)
}

// LoadCursor loads a cursor
func LoadCursor(hInst HMODULE, cursorName uintptr) HCURSOR {
	ret, _, _ := procLoadCursorW.Call(uintptr(hInst), cursorName)
	return HCURSOR(ret)
}

// SetCursor sets the cursor shape
func SetCursor(cursor HCURSOR) HCURSOR {
	ret, _, _ := procSetCursor.Call(uintptr(cursor))
	return HCURSOR(ret)
}

// GetSystemMetrics retrieves a system metric
func GetSystemMetrics(index int32) int32 {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(ret)
}

// SetTimer creates a timer
func SetTimer(hwnd HWND, idEvent uintptr, elapse uint32, timerFunc uintptr) uintptr {
	ret, _, _ := procSetTimer.Call(
		uintptr(hwnd), idEvent, uintptr(elapse), timerFunc,
	)
	return ret
}

// KillTimer destroys a timer
func KillTimer(hwnd HWND, idEvent uintptr) BOOL {
	ret, _, _ := procKillTimer.Call(uintptr(hwnd), idEvent)
	return BOOL(ret)
}

// TrackMouseEvent posts messages when the mouse pointer leaves a window
func TrackMouseEvent(tme *TRACKMOUSEEVENT) BOOL {
	ret, _, _ := procTrackMouseEvent.Call(uintptr(unsafe.Pointer(tme)))
	return BOOL(ret)
}

// GetDC retrieves a device context for the client area
func GetDC(hwnd HWND) HDC {
	ret, _, _ := procGetDC.Call(uintptr(hwnd))
	return HDC(ret)
}

// ReleaseDC releases a device context
func ReleaseDC(hwnd HWND, hdc HDC) int32 {
	ret, _, _ := procReleaseDC.Call(uintptr(hwnd), uintptr(hdc))
	return int32(ret)
}

// GetWindowDC retrieves a device context for the window
func GetWindowDC(hwnd HWND) HDC {
	ret, _, _ := procGetWindowDC.Call(uintptr(hwnd))
	return HDC(ret)
}

// EnableWindow enables or disables mouse and keyboard input
func EnableWindow(hwnd HWND, enable BOOL) BOOL {
	ret, _, _ := procEnableWindow.Call(uintptr(hwnd), uintptr(enable))
	return BOOL(ret)
}

// IsWindowEnabled checks if a window is enabled
func IsWindowEnabled(hwnd HWND) BOOL {
	ret, _, _ := procIsWindowEnabled.Call(uintptr(hwnd))
	return BOOL(ret)
}

// IsWindowVisible checks if a window is visible
func IsWindowVisible(hwnd HWND) BOOL {
	ret, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return BOOL(ret)
}

// SetFocus sets the keyboard focus to the specified window
func SetFocus(hwnd HWND) HWND {
	ret, _, _ := procSetFocus.Call(uintptr(hwnd))
	return HWND(ret)
}

// GetFocus retrieves the window with the keyboard focus
func GetFocus() HWND {
	ret, _, _ := procGetFocus.Call()
	return HWND(ret)
}

// SetForegroundWindow brings the thread that created the specified window
func SetForegroundWindow(hwnd HWND) BOOL {
	ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	return BOOL(ret)
}

// GetForegroundWindow retrieves the foreground window
func GetForegroundWindow() HWND {
	ret, _, _ := procGetForegroundWindow.Call()
	return HWND(ret)
}

// SetActiveWindow activates a window
func SetActiveWindow(hwnd HWND) HWND {
	ret, _, _ := procSetActiveWindow.Call(uintptr(hwnd))
	return HWND(ret)
}

// SetCapture captures the mouse
func SetCapture(hwnd HWND) HWND {
	ret, _, _ := procSetCapture.Call(uintptr(hwnd))
	return HWND(ret)
}

// ReleaseCapture releases the mouse capture
func ReleaseCapture() BOOL {
	ret, _, _ := procReleaseCapture.Call()
	return BOOL(ret)
}

// ClientToScreen converts client coordinates to screen coordinates
func ClientToScreen(hwnd HWND, pt *POINT) BOOL {
	ret, _, _ := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(pt)))
	return BOOL(ret)
}

// ScreenToClient converts screen coordinates to client coordinates
func ScreenToClient(hwnd HWND, pt *POINT) BOOL {
	ret, _, _ := procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(pt)))
	return BOOL(ret)
}

// GetCursorPos retrieves the cursor position in screen coordinates
func GetCursorPos(pt *POINT) BOOL {
	ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(pt)))
	return BOOL(ret)
}

// MapWindowPoints converts points from one coordinate space to another
func MapWindowPoints(hwndFrom, hwndTo HWND, pts *POINT, count uint32) int32 {
	ret, _, _ := procMapWindowPoints.Call(
		uintptr(hwndFrom), uintptr(hwndTo),
		uintptr(unsafe.Pointer(pts)), uintptr(count),
	)
	return int32(ret)
}

// GetParent retrieves the specified window's parent
func GetParent(hwnd HWND) HWND {
	ret, _, _ := procGetParent.Call(uintptr(hwnd))
	return HWND(ret)
}

// SetParent changes the parent window
func SetParent(hwndChild, hwndNewParent HWND) HWND {
	ret, _, _ := procSetParent.Call(uintptr(hwndChild), uintptr(hwndNewParent))
	return HWND(ret)
}

// IsWindow determines whether the specified window handle identifies an existing window
func IsWindow(hwnd HWND) BOOL {
	ret, _, _ := procIsWindow.Call(uintptr(hwnd))
	return BOOL(ret)
}

// MessageBox displays a modal dialog box
func MessageBox(hwnd HWND, text, caption *uint16, msgType UINT) int32 {
	ret, _, _ := procMessageBoxW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(caption)),
		uintptr(msgType),
	)
	return int32(ret)
}

// AdjustWindowRectEx calculates the required size of the window rectangle
func AdjustWindowRectEx(rect *RECT, style DWORD, menu BOOL, exStyle DWORD) BOOL {
	ret, _, _ := procAdjustWindowRectEx.Call(
		uintptr(unsafe.Pointer(rect)), uintptr(style),
		uintptr(menu), uintptr(exStyle),
	)
	return BOOL(ret)
}

// GetAsyncKeyState determines whether a key is up or down
func GetAsyncKeyState(vKey int32) int16 {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vKey))
	return int16(ret)
}

// GetKeyState retrieves the status of the specified virtual key
func GetKeyState(vKey int32) int16 {
	ret, _, _ := procGetKeyState.Call(uintptr(vKey))
	return int16(ret)
}

// PeekMessage checks for a message without waiting
func PeekMessage(msg *MSG, hwnd HWND, msgFilterMin, msgFilterMax UINT, removeMsg UINT) BOOL {
	ret, _, _ := procPeekMessageW.Call(
		uintptr(unsafe.Pointer(msg)), uintptr(hwnd),
		uintptr(msgFilterMin), uintptr(msgFilterMax), uintptr(removeMsg),
	)
	return BOOL(ret)
}

// WaitMessage yields control to other threads
func WaitMessage() BOOL {
	ret, _, _ := procWaitMessage.Call()
	return BOOL(ret)
}

// SetLayeredWindowAttributes sets the opacity and transparency color key
func SetLayeredWindowAttributes(hwnd HWND, crKey uint32, bAlpha byte, dwFlags DWORD) BOOL {
	ret, _, _ := procSetLayeredWindowAttributes.Call(
		uintptr(hwnd), uintptr(crKey), uintptr(bAlpha), uintptr(dwFlags),
	)
	return BOOL(ret)
}

// RegisterWindowMessage defines a new window message
func RegisterWindowMessage(msgName *uint16) UINT {
	ret, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(msgName)))
	return UINT(ret)
}

// GetMessagePos returns the cursor position for the last message
func GetMessagePos() DWORD {
	ret, _, _ := procGetMessagePos.Call()
	return DWORD(ret)
}

// DPI functions
func SetProcessDpiAwarenessContext(value uintptr) BOOL {
	ret, _, _ := procSetProcessDpiAwarenessContext.Call(value)
	return BOOL(ret)
}

func GetDpiForWindow(hwnd HWND) UINT {
	ret, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	return UINT(ret)
}

func GetDpiForSystem() UINT {
	ret, _, _ := procGetDpiForSystem.Call()
	return UINT(ret)
}

func EnableNonClientDpiScaling(hwnd HWND) BOOL {
	ret, _, _ := procEnableNonClientDpiScaling.Call(uintptr(hwnd))
	return BOOL(ret)
}

func SetProcessDPIAware() BOOL {
	ret, _, _ := procSetProcessDPIAware.Call()
	return BOOL(ret)
}

// Menu functions
func CreateMenu() HMENU {
	ret, _, _ := procCreateMenu.Call()
	return HMENU(ret)
}

func CreatePopupMenu() HMENU {
	ret, _, _ := procCreatePopupMenu.Call()
	return HMENU(ret)
}

func AppendMenu(hMenu HMENU, flags UINT, idNewItem uintptr, newItem *uint16) BOOL {
	ret, _, _ := procAppendMenuW.Call(
		uintptr(hMenu), uintptr(flags), idNewItem, uintptr(unsafe.Pointer(newItem)),
	)
	return BOOL(ret)
}

func InsertMenu(hMenu HMENU, position UINT, flags UINT, idNewItem uintptr, newItem *uint16) BOOL {
	ret, _, _ := procInsertMenuW.Call(
		uintptr(hMenu), uintptr(position), uintptr(flags),
		idNewItem, uintptr(unsafe.Pointer(newItem)),
	)
	return BOOL(ret)
}

func SetMenu(hwnd HWND, hMenu HMENU) BOOL {
	ret, _, _ := procSetMenu.Call(uintptr(hwnd), uintptr(hMenu))
	return BOOL(ret)
}

func GetMenu(hwnd HWND) HMENU {
	ret, _, _ := procGetMenu.Call(uintptr(hwnd))
	return HMENU(ret)
}

func DestroyMenu(hMenu HMENU) BOOL {
	ret, _, _ := procDestroyMenu.Call(uintptr(hMenu))
	return BOOL(ret)
}

func TrackPopupMenu(hMenu HMENU, flags UINT, x, y int32, reserved int32, hwnd HWND, prc *RECT) BOOL {
	ret, _, _ := procTrackPopupMenu.Call(
		uintptr(hMenu), uintptr(flags),
		uintptr(x), uintptr(y), uintptr(reserved),
		uintptr(hwnd), uintptr(unsafe.Pointer(prc)),
	)
	return BOOL(ret)
}

func EnableMenuItem(hMenu HMENU, idEnableItem UINT, enable UINT) BOOL {
	ret, _, _ := procEnableMenuItem.Call(
		uintptr(hMenu), uintptr(idEnableItem), uintptr(enable),
	)
	return BOOL(ret)
}

func CheckMenuItem(hMenu HMENU, idCheckItem UINT, check UINT) DWORD {
	ret, _, _ := procCheckMenuItem.Call(
		uintptr(hMenu), uintptr(idCheckItem), uintptr(check),
	)
	return DWORD(ret)
}

func GetMenuItemCount(hMenu HMENU) int32 {
	ret, _, _ := procGetMenuItemCount.Call(uintptr(hMenu))
	return int32(ret)
}

func GetMenuItemID(hMenu HMENU, pos int32) UINT {
	ret, _, _ := procGetMenuItemID.Call(uintptr(hMenu), uintptr(pos))
	return UINT(ret)
}

func GetSubMenu(hMenu HMENU, pos int32) HMENU {
	ret, _, _ := procGetSubMenu.Call(uintptr(hMenu), uintptr(pos))
	return HMENU(ret)
}

func RemoveMenu(hMenu HMENU, position UINT, flags UINT) BOOL {
	ret, _, _ := procRemoveMenu.Call(
		uintptr(hMenu), uintptr(position), uintptr(flags),
	)
	return BOOL(ret)
}

func DeleteMenu(hMenu HMENU, position UINT, flags UINT) BOOL {
	ret, _, _ := procDeleteMenu.Call(
		uintptr(hMenu), uintptr(position), uintptr(flags),
	)
	return BOOL(ret)
}

func GetWindowDC2(hwnd HWND) HDC {
	ret, _, _ := procGetWindowDC.Call(uintptr(hwnd))
	return HDC(ret)
}

// LOWORD extracts the low-order word from a value.
func LOWORD(v uint32) uint16 { return uint16(v & 0xFFFF) }

// HIWORD extracts the high-order word from a value.
func HIWORD(v uint32) uint16 { return uint16((v >> 16) & 0xFFFF) }

// FillRect fills a rectangle using the specified brush (user32).
func FillRect(hdc HDC, rect *RECT, brush HBRUSH) int32 {
	ret, _, _ := procFillRect.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(rect)), uintptr(brush),
	)
	return int32(ret)
}

// FrameRect draws a border around the specified rectangle (user32).
func FrameRect(hdc HDC, rect *RECT, brush HBRUSH) int32 {
	ret, _, _ := procFrameRect.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(rect)), uintptr(brush),
	)
	return int32(ret)
}

// DrawText draws formatted text in the specified rectangle (user32).
func DrawText(hdc HDC, text *uint16, count int32, rect *RECT, format UINT) int32 {
	ret, _, _ := procDrawTextW.Call(
		uintptr(hdc), uintptr(unsafe.Pointer(text)), uintptr(count),
		uintptr(unsafe.Pointer(rect)), uintptr(format),
	)
	return int32(ret)
}
