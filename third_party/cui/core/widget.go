package core

import (
	"cui/win32"
	"sync"
	"unsafe"
)

// Widget is the interface all controls implement
type Widget interface {
	Bounds() Rect
	SetBounds(r Rect)
	SetPosition(x, y int32)
	SetSize(w, h int32)
	Visible() bool
	SetVisible(v bool)
	Enabled() bool
	SetEnabled(v bool)
	Handle() win32.HWND
	SetFont(f *Font)
	Invalidate()
	Paint(canvas *Canvas)
	Destroy()
}

// Global widget registry: HWND -> handler
var (
	widgetRegistry   = make(map[win32.HWND]Widget)
	widgetRegistryMu sync.RWMutex
)

func registerWidget(hwnd win32.HWND, w Widget) {
	widgetRegistryMu.Lock()
	widgetRegistry[hwnd] = w
	widgetRegistryMu.Unlock()
}

func unregisterWidget(hwnd win32.HWND) {
	widgetRegistryMu.Lock()
	delete(widgetRegistry, hwnd)
	widgetRegistryMu.Unlock()
}

func getWidget(hwnd win32.HWND) Widget {
	widgetRegistryMu.RLock()
	defer widgetRegistryMu.RUnlock()
	return widgetRegistry[hwnd]
}

// Widget unique ID counter
var widgetIDCounter uint32 = 1000

func nextWidgetID() uint32 {
	widgetIDCounter++
	return widgetIDCounter
}

// BaseWidget is the common base for all widgets
type BaseWidget struct {
	hwnd       win32.HWND
	parentHwnd win32.HWND
	id         uint32
	bounds     Rect
	visible    bool
	enabled    bool
	font       *Font
	isHovering bool
	isPressed  bool
	isFocused  bool
	tracking   bool

	// Event callbacks
	onClick        ClickCallback
	onDoubleClick  ClickCallback
	onMouseMove    MouseCallback
	onMouseDown    MouseCallback
	onMouseUp      MouseCallback
	onMouseEnter   MouseCallback
	onMouseLeave   MouseCallback
	onMouseWheel   ScrollCallback
	onKeyDown      KeyCallback
	onKeyUp        KeyCallback
	onChar         KeyCallback
	onPaint        PaintCallback
	onResize       ResizeCallback
	onFocus        FocusCallback
	onBlur         FocusCallback
}

// NewBaseWidget creates a new BaseWidget as a child of the given parent
func NewBaseWidget(parentHwnd win32.HWND, className string, style, exStyle uint32, x, y, w, h int32) *BaseWidget {
	bw := &BaseWidget{
		parentHwnd: parentHwnd,
		id:         nextWidgetID(),
		bounds:     Rect{X: x, Y: y, Width: w, Height: h},
		visible:    true,
		enabled:    true,
	}

	hwnd := win32.CreateWindowEx(
		win32.DWORD(exStyle),
		win32.UTF16PtrFromString(className),
		nil,
		win32.DWORD(style)|win32.WS_CHILD|win32.WS_VISIBLE,
		x, y, w, h,
		parentHwnd,
		win32.HMENU(win32.Uintptr(unsafe.Pointer(uintptr(bw.id)))),
		0, nil,
	)

	if hwnd == 0 {
		return nil
	}

	bw.hwnd = hwnd
	registerWidget(hwnd, bw)
	return bw
}

// NewBaseWidgetFromHWND wraps an existing HWND into a BaseWidget value.
//
// IMPORTANT: callers that embed BaseWidget by value MUST re-bind the embedded
// field after assignment, otherwise paint/click callbacks live on a discarded
// copy while the registry still points at the temporary:
//
//	w.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
//	w.SetOnPaint(w.paint)
//	core.BindBaseWidget(&w.BaseWidget)
func NewBaseWidgetFromHWND(hwnd win32.HWND) *BaseWidget {
	bw := &BaseWidget{
		hwnd:    hwnd,
		id:      nextWidgetID(),
		visible: true,
		enabled: true,
	}
	var rc win32.RECT
	win32.GetClientRect(hwnd, &rc)
	// Client rect is local (0,0,w,h). Keep width/height; position is set via SetBounds.
	bw.bounds = Rect{X: 0, Y: 0, Width: int32(rc.Right - rc.Left), Height: int32(rc.Bottom - rc.Top)}
	// Temporary registration so HWND is never orphaned; BindBaseWidget replaces it.
	registerWidget(hwnd, bw)
	return bw
}

// BindBaseWidget re-registers hwnd → *BaseWidget for the embedded field of a
// concrete widget (Label/Button/…). Call after embedding and setting callbacks.
func BindBaseWidget(bw *BaseWidget) {
	if bw == nil || bw.hwnd == 0 {
		return
	}
	registerWidget(bw.hwnd, bw)
}

// RegisterWidget exposes HWND→Widget registration for advanced hosts.
func RegisterWidget(hwnd win32.HWND, w Widget) {
	registerWidget(hwnd, w)
}

// Bounds returns the widget bounds
func (w *BaseWidget) Bounds() Rect { return w.bounds }

// SetBounds sets the widget bounds
func (w *BaseWidget) SetBounds(r Rect) {
	w.bounds = r
	if w.hwnd != 0 {
		win32.MoveWindow(w.hwnd, r.X, r.Y, r.Width, r.Height, 1)
	}
}

// SetPosition sets the widget position
func (w *BaseWidget) SetPosition(x, y int32) {
	w.bounds.X = x
	w.bounds.Y = y
	if w.hwnd != 0 {
		win32.MoveWindow(w.hwnd, x, y, w.bounds.Width, w.bounds.Height, 1)
	}
}

// SetSize sets the widget size
func (w *BaseWidget) SetSize(width, height int32) {
	w.bounds.Width = width
	w.bounds.Height = height
	if w.hwnd != 0 {
		win32.MoveWindow(w.hwnd, w.bounds.X, w.bounds.Y, width, height, 1)
	}
}

// Visible returns visibility state
func (w *BaseWidget) Visible() bool { return w.visible }

// SetVisible sets visibility
func (w *BaseWidget) SetVisible(v bool) {
	w.visible = v
	if w.hwnd != 0 {
		cmd := win32.SW_HIDE
		if v {
			cmd = win32.SW_SHOW
		}
		win32.ShowWindow(w.hwnd, int32(cmd))
	}
}

// Enabled returns enabled state
func (w *BaseWidget) Enabled() bool { return w.enabled }

// SetEnabled sets enabled state
func (w *BaseWidget) SetEnabled(v bool) {
	w.enabled = v
	if w.hwnd != 0 {
		flag := win32.FALSE
		if v {
			flag = win32.TRUE
		}
		win32.EnableWindow(w.hwnd, flag)
	}
}

// Handle returns the native window handle
func (w *BaseWidget) Handle() win32.HWND { return w.hwnd }

// SetFont sets the widget font
func (w *BaseWidget) SetFont(f *Font) {
	w.font = f
	if w.hwnd != 0 && f != nil {
		win32.SendMessage(w.hwnd, win32.WM_SETFONT, win32.WPARAM(f.Handle), 1)
	}
}

// Font returns the widget font.
func (w *BaseWidget) Font() *Font { return w.font }

// IsPressed reports left-button pressed state.
func (w *BaseWidget) IsPressed() bool { return w.isPressed }

// IsHovering reports mouse hover state.
func (w *BaseWidget) IsHovering() bool { return w.isHovering }

// IsFocused reports keyboard focus state.
func (w *BaseWidget) IsFocused() bool { return w.isFocused }

// OnClick returns the click callback (for subclasses).
func (w *BaseWidget) OnClick() ClickCallback { return w.onClick }

// Invalidate schedules a repaint
func (w *BaseWidget) Invalidate() {
	if w.hwnd != 0 {
		win32.InvalidateRect(w.hwnd, nil, win32.TRUE)
	}
}

// Paint is overridden by concrete widgets
func (w *BaseWidget) Paint(canvas *Canvas) {}

// Destroy destroys the widget
func (w *BaseWidget) Destroy() {
	unregisterWidget(w.hwnd)
	if w.hwnd != 0 {
		win32.DestroyWindow(w.hwnd)
		w.hwnd = 0
	}
}

// SetOnClick sets the click callback
func (w *BaseWidget) SetOnClick(fn ClickCallback) { w.onClick = fn }

// SetOnDoubleClick sets the double-click callback
func (w *BaseWidget) SetOnDoubleClick(fn ClickCallback) { w.onDoubleClick = fn }

// SetOnMouseMove sets the mouse move callback
func (w *BaseWidget) SetOnMouseMove(fn MouseCallback) { w.onMouseMove = fn }

// SetOnMouseDown sets the mouse down callback
func (w *BaseWidget) SetOnMouseDown(fn MouseCallback) { w.onMouseDown = fn }

// SetOnMouseUp sets the mouse up callback
func (w *BaseWidget) SetOnMouseUp(fn MouseCallback) { w.onMouseUp = fn }

// SetOnMouseEnter sets the mouse enter callback
func (w *BaseWidget) SetOnMouseEnter(fn MouseCallback) { w.onMouseEnter = fn }

// SetOnMouseLeave sets the mouse leave callback
func (w *BaseWidget) SetOnMouseLeave(fn MouseCallback) { w.onMouseLeave = fn }

// SetOnMouseWheel sets the mouse wheel callback
func (w *BaseWidget) SetOnMouseWheel(fn ScrollCallback) { w.onMouseWheel = fn }

// SetOnKeyDown sets the key down callback
func (w *BaseWidget) SetOnKeyDown(fn KeyCallback) { w.onKeyDown = fn }

// SetOnKeyUp sets the key up callback
func (w *BaseWidget) SetOnKeyUp(fn KeyCallback) { w.onKeyUp = fn }

// SetOnChar sets the char callback
func (w *BaseWidget) SetOnChar(fn KeyCallback) { w.onChar = fn }

// SetOnPaint sets the paint callback
func (w *BaseWidget) SetOnPaint(fn PaintCallback) { w.onPaint = fn }

// SetOnResize sets the resize callback
func (w *BaseWidget) SetOnResize(fn ResizeCallback) { w.onResize = fn }

// SetOnFocus sets the focus gain callback
func (w *BaseWidget) SetOnFocus(fn FocusCallback) { w.onFocus = fn }

// SetOnBlur sets the focus loss callback
func (w *BaseWidget) SetOnBlur(fn FocusCallback) { w.onBlur = fn }

// startTracking starts mouse tracking if not already tracking
func (w *BaseWidget) startTracking() {
	if !w.tracking {
		tme := win32.TRACKMOUSEEVENT{
			CbSize:    24, // sizeof(TRACKMOUSEEVENT)
			DwFlags:   win32.TME_LEAVE,
			HwndTrack: w.hwnd,
		}
		win32.TrackMouseEvent(&tme)
		w.tracking = true
	}
}

// handleMouseMessage processes mouse messages for the widget
func (w *BaseWidget) handleMouseMessage(msg uint32, wparam, lparam uintptr) uintptr {
	x := int32(int16(win32.LOWORD(uint32(lparam))))
	y := int32(int16(win32.HIWORD(uint32(lparam))))

	mod := KeyModifiers(0)
	if wparam&win32.MK_SHIFT != 0 {
		mod |= KeyModShift
	}
	if wparam&win32.MK_CONTROL != 0 {
		mod |= KeyModControl
	}

	event := MouseEvent{X: x, Y: y, Modifiers: mod}

	switch msg {
	case win32.WM_MOUSEMOVE:
		if !w.isHovering {
			w.isHovering = true
			w.startTracking()
			if w.onMouseEnter != nil {
				w.onMouseEnter(event)
			}
			w.Invalidate()
		}
		if w.onMouseMove != nil {
			w.onMouseMove(event)
		}

	case win32.WM_MOUSELEAVE:
		w.isHovering = false
		w.tracking = false
		if w.isPressed {
			w.isPressed = false
		}
		if w.onMouseLeave != nil {
			w.onMouseLeave(event)
		}
		w.Invalidate()

	case win32.WM_LBUTTONDOWN:
		w.isPressed = true
		event.Button = MouseButtonLeft
		win32.SetCapture(w.hwnd)
		if w.onMouseDown != nil {
			w.onMouseDown(event)
		}
		w.Invalidate()

	case win32.WM_LBUTTONUP:
		if w.isPressed {
			w.isPressed = false
			win32.ReleaseCapture()
			event.Button = MouseButtonLeft
			if w.onMouseUp != nil {
				w.onMouseUp(event)
			}
			// lparam is client-relative to this HWND (0..width/height), not parent coords.
			inClient := x >= 0 && y >= 0 && x < w.bounds.Width && y < w.bounds.Height
			if inClient && w.onClick != nil {
				w.onClick()
			}
			w.Invalidate()
		}

	case win32.WM_LBUTTONDBLCLK:
		event.Button = MouseButtonLeft
		if w.onDoubleClick != nil {
			w.onDoubleClick()
		}

	case win32.WM_RBUTTONDOWN:
		event.Button = MouseButtonRight
		if w.onMouseDown != nil {
			w.onMouseDown(event)
		}

	case win32.WM_RBUTTONUP:
		event.Button = MouseButtonRight
		if w.onMouseUp != nil {
			w.onMouseUp(event)
		}

	case win32.WM_MOUSEWHEEL:
		delta := int32(int16(win32.HIWORD(uint32(wparam))))
		se := ScrollEvent{DeltaY: delta}
		if w.onMouseWheel != nil {
			w.onMouseWheel(se)
		}
	}

	return 0
}

// WndProc processes messages for custom-drawn widgets
func (w *BaseWidget) WndProc(msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case win32.WM_PAINT:
		var ps win32.PAINTSTRUCT
		hdc := win32.BeginPaint(w.hwnd, &ps)
		if hdc != 0 {
			cw, ch := w.bounds.Width, w.bounds.Height
			var rc win32.RECT
			if win32.GetClientRect(w.hwnd, &rc) != 0 {
				if int32(rc.Right) > 0 {
					cw = int32(rc.Right)
				}
				if int32(rc.Bottom) > 0 {
					ch = int32(rc.Bottom)
				}
			}
			// Keep bounds size in sync with real client area for hit-testing.
			if cw > 0 {
				w.bounds.Width = cw
			}
			if ch > 0 {
				w.bounds.Height = ch
			}
			canvas := NewCanvas(hdc, w.hwnd, cw, ch)
			w.Paint(canvas)
			if w.onPaint != nil {
				w.onPaint(canvas)
			}
			win32.EndPaint(w.hwnd, &ps)
		}
		return 0

	case win32.WM_ERASEBKGND:
		return 1 // prevent flicker

	case win32.WM_MOUSEMOVE, win32.WM_LBUTTONDOWN, win32.WM_LBUTTONUP,
		win32.WM_LBUTTONDBLCLK, win32.WM_RBUTTONDOWN, win32.WM_RBUTTONUP,
		win32.WM_MOUSEWHEEL, win32.WM_MOUSELEAVE:
		return w.handleMouseMessage(msg, wparam, lparam)

	case win32.WM_KEYDOWN:
		ke := KeyEvent{Key: uint32(wparam), IsRepeat: lparam&0x40000000 != 0}
		if w.onKeyDown != nil {
			w.onKeyDown(ke)
		}
		return 0

	case win32.WM_KEYUP:
		ke := KeyEvent{Key: uint32(wparam)}
		if w.onKeyUp != nil {
			w.onKeyUp(ke)
		}
		return 0

	case win32.WM_CHAR:
		ke := KeyEvent{Char: rune(wparam), Key: uint32(wparam)}
		if w.onChar != nil {
			w.onChar(ke)
		}
		return 0

	case win32.WM_SETFOCUS:
		w.isFocused = true
		if w.onFocus != nil {
			w.onFocus(FocusEvent{Gained: true})
		}
		w.Invalidate()
		return 0

	case win32.WM_KILLFOCUS:
		w.isFocused = false
		if w.onBlur != nil {
			w.onBlur(FocusEvent{Gained: false})
		}
		w.Invalidate()
		return 0

	case win32.WM_GETDLGCODE:
		// Allow Tab key to be processed by the dialog manager
		return win32.DLGC_WANTALLKEYS
	}

	return uintptr(win32.DefWindowProc(win32.HWND(w.hwnd), win32.UINT(msg), win32.WPARAM(wparam), win32.LPARAM(lparam)))
}

// DLGC constants for WM_GETDLGCODE
const (
	DLGC_WANTARROWS     = 0x0001
	DLGC_WANTTAB        = 0x0002
	DLGC_WANTALLKEYS    = 0x0004
	DLGC_WANTMESSAGE    = 0x0004
	DLGC_HASSETSEL      = 0x0008
	DLGC_DEFPUSHBUTTON  = 0x0010
	DLGC_UNDEFPUSHBUTTON = 0x0020
	DLGC_RADIOBUTTON    = 0x0040
	DLGC_WANTCHARS      = 0x0080
	DLGC_STATIC         = 0x0100
	DLGC_BUTTON         = 0x2000
)

// LOWORD extracts the low word from a DWORD
func LOWORD(v uint32) uint16 { return uint16(v & 0xFFFF) }
func HIWORD(v uint32) uint16 { return uint16((v >> 16) & 0xFFFF) }
