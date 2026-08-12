package core

import (
	"cui/win32"
	"unsafe"
)

// Layout interface for window layouts
type Layout interface {
	SetPadding(p int32)
	SetSpacing(s int32)
	LayoutChildren(containerBounds Rect, widgets []Widget)
}

// Window represents a top-level window
type Window struct {
	hwnd       win32.HWND
	title      string
	width      int32
	height     int32
	minWidth   int32
	minHeight  int32
	maxWidth   int32
	maxHeight  int32
	layout     Layout
	widgets    []Widget
	bgColor    Color
	app        *Window // reference to app (stored as parent)
	font       *Font
	onClose    CloseCallback
	onResize   ResizeCallback
	onPaint    PaintCallback
	onKeyDown  KeyCallback
	modal      bool
	modalDone  bool
}

// newWindow creates a new top-level window
func newWindow(app *App, title string, width, height int32) *Window {
	w := &Window{
		title:    title,
		width:    width,
		height:   height,
		bgColor:  ColorWhite,
		font:     DefaultFont(),
	}

	// Adjust window rect for style
	rc := win32.RECT{
		Left:   0,
		Top:    0,
		Right:  win32.LONG(width),
		Bottom: win32.LONG(height),
	}
	win32.AdjustWindowRectEx(&rc, win32.WS_OVERLAPPEDWINDOW, 0, 0)

	adjustedW := int32(rc.Right - rc.Left)
	adjustedH := int32(rc.Bottom - rc.Top)

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.MainClassName()),
		win32.UTF16PtrFromString(title),
		win32.WS_OVERLAPPEDWINDOW,
		win32.CW_USEDEFAULT, win32.CW_USEDEFAULT,
		adjustedW, adjustedH,
		0, 0, app.Instance(), nil,
	)

	if hwnd == 0 {
		return nil
	}

	w.hwnd = hwnd
	registerWindowProc(hwnd, w.wndProc)
	registerWidget(hwnd, w)

	// Set default font
	if w.font != nil {
		win32.SendMessage(hwnd, win32.WM_SETFONT, win32.WPARAM(w.font.Handle), 1)
	}

	return w
}

// Handle returns the window handle
func (w *Window) Handle() win32.HWND {
	return w.hwnd
}

// Title returns the window title
func (w *Window) Title() string {
	return w.title
}

// SetTitle sets the window title
func (w *Window) SetTitle(title string) {
	w.title = title
	win32.SetWindowText(w.hwnd, win32.UTF16PtrFromString(title))
}

// Bounds returns the window bounds
func (w *Window) Bounds() Rect {
	var rc win32.RECT
	win32.GetWindowRect(w.hwnd, &rc)
	return FromRECT(rc)
}

// ClientBounds returns the client area bounds
func (w *Window) ClientBounds() Rect {
	var rc win32.RECT
	win32.GetClientRect(w.hwnd, &rc)
	return FromRECT(rc)
}

// SetBounds sets the window bounds
func (w *Window) SetBounds(r Rect) {
	win32.MoveWindow(w.hwnd, r.X, r.Y, r.Width, r.Height, 1)
}

// SetSize sets the window size
func (w *Window) SetSize(width, height int32) {
	w.width = width
	w.height = height
	rc := win32.RECT{Right: win32.LONG(width), Bottom: win32.LONG(height)}
	win32.AdjustWindowRectEx(&rc, win32.WS_OVERLAPPEDWINDOW, 0, 0)
	win32.MoveWindow(w.hwnd, 0, 0, int32(rc.Right-rc.Left), int32(rc.Bottom-rc.Top), 1)
}

// SetPosition sets the window position
func (w *Window) SetPosition(x, y int32) {
	win32.SetWindowPos(w.hwnd, 0, x, y, 0, 0, win32.SWP_NOSIZE|win32.SWP_NOZORDER)
}

// CenterScreen centers the window on the screen
func (w *Window) CenterScreen() {
	screenW := win32.GetSystemMetrics(win32.SM_CXSCREEN)
	screenH := win32.GetSystemMetrics(win32.SM_CYSCREEN)
	var rc win32.RECT
	win32.GetWindowRect(w.hwnd, &rc)
	ww := int32(rc.Right - rc.Left)
	wh := int32(rc.Bottom - rc.Top)
	x := (screenW - ww) / 2
	y := (screenH - wh) / 2
	win32.SetWindowPos(w.hwnd, 0, x, y, 0, 0, win32.SWP_NOSIZE|win32.SWP_NOZORDER)
}

// SetMinSize sets the minimum window size
func (w *Window) SetMinSize(width, height int32) {
	w.minWidth = width
	w.minHeight = height
}

// SetMaxSize sets the maximum window size
func (w *Window) SetMaxSize(width, height int32) {
	w.maxWidth = width
	w.maxHeight = height
}

// SetLayout sets the window layout
func (w *Window) SetLayout(l Layout) {
	w.layout = l
	if l != nil {
		w.doLayout()
	}
}

// Layout returns the window layout
func (w *Window) Layout() Layout {
	return w.layout
}

// SetBackgroundColor sets the window background color
func (w *Window) SetBackgroundColor(c Color) {
	w.bgColor = c
	win32.InvalidateRect(w.hwnd, nil, win32.TRUE)
}

// Font returns the window font
func (w *Window) Font() *Font {
	return w.font
}

// SetFont sets the window default font
func (w *Window) SetFont(f *Font) {
	w.font = f
}

// Show shows the window
func (w *Window) Show() {
	// Layout before first paint so vbox/hbox positions are correct without a resize.
	w.doLayout()
	win32.ShowWindow(w.hwnd, win32.SW_SHOW)
	win32.ShowWindow(w.hwnd, win32.SW_RESTORE)
	win32.SetForegroundWindow(w.hwnd)
	win32.UpdateWindow(w.hwnd)
}

// Hide hides the window
func (w *Window) Hide() {
	win32.ShowWindow(w.hwnd, win32.SW_HIDE)
}

// Close closes the window
func (w *Window) Close() {
	win32.DestroyWindow(w.hwnd)
}

// Minimize minimizes the window
func (w *Window) Minimize() {
	win32.ShowWindow(w.hwnd, win32.SW_MINIMIZE)
}

// Maximize maximizes the window
func (w *Window) Maximize() {
	win32.ShowWindow(w.hwnd, win32.SW_MAXIMIZE)
}

// Restore restores the window from minimized/maximized state
func (w *Window) Restore() {
	win32.ShowWindow(w.hwnd, win32.SW_RESTORE)
}

// SetTopMost sets or clears the topmost flag
func (w *Window) SetTopMost(topmost bool) {
	flag := win32.HWND_NOTOPMOST
	if topmost {
		flag = win32.HWND_TOPMOST
	}
	win32.SetWindowPos(w.hwnd, flag, 0, 0, 0, 0, win32.SWP_NOMOVE|win32.SWP_NOSIZE)
}

// Visible returns whether the window is visible
func (w *Window) Visible() bool {
	return win32.IsWindowVisible(w.hwnd) != 0
}

// Invalidate schedules a repaint
func (w *Window) Invalidate() {
	win32.InvalidateRect(w.hwnd, nil, win32.TRUE)
}

// SetOnClose sets the close callback
func (w *Window) SetOnClose(fn CloseCallback) {
	w.onClose = fn
}

// SetOnResize sets the resize callback
func (w *Window) SetOnResize(fn ResizeCallback) {
	w.onResize = fn
}

// SetOnPaint sets the paint callback
func (w *Window) SetOnPaint(fn PaintCallback) {
	w.onPaint = fn
}

// SetOnKeyDown sets the key down callback
func (w *Window) SetOnKeyDown(fn KeyCallback) {
	w.onKeyDown = fn
}

// SetBounds2 is an alias for SetBounds
func (w *Window) SetBounds2(x, y, width, height int32) {
	win32.MoveWindow(w.hwnd, x, y, width, height, 1)
}

// Visible2 is an alias for Visible
func (w *Window) Visible2() bool { return w.Visible() }

// SetVisible shows or hides the window
func (w *Window) SetVisible(v bool) {
	if v {
		w.Show()
	} else {
		w.Hide()
	}
}

// Enabled returns whether the window is enabled
func (w *Window) Enabled() bool {
	return win32.IsWindowEnabled(w.hwnd) != 0
}

// SetEnabled enables or disables the window
func (w *Window) SetEnabled(v bool) {
	flag := win32.FALSE
	if v {
		flag = win32.TRUE
	}
	win32.EnableWindow(w.hwnd, flag)
}

// ShowModal shows the window as a modal dialog
func (w *Window) ShowModal() {
	w.modal = true
	w.modalDone = false

	// Disable parent window
	parentHwnd := win32.GetWindowLongPtr(w.hwnd, win32.GWLP_HWNDPARENT)
	if parentHwnd != 0 {
		win32.EnableWindow(win32.HWND(parentHwnd), win32.FALSE)
	}

	w.Show()
	win32.SetForegroundWindow(w.hwnd)

	// Run modal message loop
	GetApp().ModalLoop(&w.modalDone)

	// Re-enable parent
	if parentHwnd != 0 {
		win32.EnableWindow(win32.HWND(parentHwnd), win32.TRUE)
		win32.SetForegroundWindow(win32.HWND(parentHwnd))
	}
}

// doLayout triggers a layout pass
func (w *Window) doLayout() {
	if w.layout != nil && len(w.widgets) > 0 {
		bounds := w.ClientBounds()
		w.layout.LayoutChildren(bounds, w.widgets)
	}
}

// wndProc is the window procedure for this window
func (w *Window) wndProc(hwnd win32.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case win32.WM_SIZE:
		w.width = int32(win32.LOWORD(uint32(lparam)))
		w.height = int32(win32.HIWORD(uint32(lparam)))
		w.doLayout()
		if w.onResize != nil {
			w.onResize(ResizeEvent{Width: w.width, Height: w.height})
		}
		return 0

	case win32.WM_PAINT:
		var ps win32.PAINTSTRUCT
		hdc := win32.BeginPaint(hwnd, &ps)
		if hdc != 0 {
			canvas := NewCanvas(hdc, hwnd, w.width, w.height)
			// Fill background
			canvas.FillRect(0, 0, w.width, w.height, w.bgColor)
			if w.onPaint != nil {
				w.onPaint(canvas)
			}
			win32.EndPaint(hwnd, &ps)
		}
		return 0

	case win32.WM_ERASEBKGND:
		return 1 // handled in WM_PAINT

	case win32.WM_CLOSE:
		if w.onClose != nil {
			if !w.onClose() {
				return 0 // prevent close
			}
		}
		if w.modal {
			w.modalDone = true
		}
		win32.DestroyWindow(hwnd)
		return 0

	case win32.WM_DESTROY:
		unregisterWidget(hwnd)
		unregisterWindowProc(hwnd)
		ReleaseWindowResources(hwnd)
		GetApp().removeWindow(hwnd)

		// If this was the main window, post quit
		if GetApp().getWindow(0) == nil {
			// Check if there are other windows
			hasOther := false
			GetApp().mu.Lock()
			for _, win := range GetApp().windows {
				if win != nil && win.hwnd != hwnd {
					hasOther = true
					break
				}
			}
			GetApp().mu.Unlock()
			if !hasOther {
				win32.PostQuitMessage(0)
			}
		}
		return 0

	case win32.WM_GETMINMAXINFO:
		if w.minWidth > 0 || w.minHeight > 0 || w.maxWidth > 0 || w.maxHeight > 0 {
			mmi := (*win32.MINMAXINFO)(unsafe.Pointer(lparam))
			if w.minWidth > 0 {
				mmi.PtMinTrackSize.X = win32.LONG(w.minWidth)
			}
			if w.minHeight > 0 {
				mmi.PtMinTrackSize.Y = win32.LONG(w.minHeight)
			}
			if w.maxWidth > 0 {
				mmi.PtMaxTrackSize.X = win32.LONG(w.maxWidth)
			}
			if w.maxHeight > 0 {
				mmi.PtMaxTrackSize.Y = win32.LONG(w.maxHeight)
			}
			return 0
		}

	case win32.WM_KEYDOWN:
		ke := KeyEvent{Key: uint32(wparam)}
		if w.onKeyDown != nil {
			w.onKeyDown(ke)
		}

	case win32.WM_DPICHANGED:
		// Handle DPI change
		newDPI := int32(win32.HIWORD(uint32(wparam)))
		_ = newDPI
		// Suggested new window rect
		newRect := (*win32.RECT)(unsafe.Pointer(lparam))
		if newRect != nil {
			win32.SetWindowPos(hwnd, 0,
				int32(newRect.Left), int32(newRect.Top),
				int32(newRect.Right-newRect.Left), int32(newRect.Bottom-newRect.Top),
				win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
		}
		return 0

	case win32.WM_COMMAND:
		// Route to child widget by ID
		id := win32.LOWORD(uint32(wparam))
		notifyCode := win32.HIWORD(uint32(lparam))
		childHwnd := win32.HWND(lparam)

		if childHwnd != 0 {
			w2 := getWidget(childHwnd)
			if w2 != nil {
				if bw, ok := w2.(*BaseWidget); ok {
					// Route button click
					if notifyCode == 0 { // BN_CLICKED
						if bw.onClick != nil {
							bw.onClick()
						}
					}
				}
			}
		}
		_ = id
		return 0

	case win32.WM_CONTEXTMENU:
		// Right-click context menu - handled by individual widgets
		break
	}

	return uintptr(win32.DefWindowProc(hwnd, win32.UINT(msg), win32.WPARAM(wparam), win32.LPARAM(lparam)))
}

// Paint implements Widget.
func (w *Window) Paint(canvas *Canvas) {}

// Visible2/Enabled2/etc are already defined on Window
// Window also implements Widget interface via its own methods

// Bounds2 returns the window client bounds (for layout purposes)
func (w *Window) ClientSize() (int32, int32) {
	var rc win32.RECT
	win32.GetClientRect(w.hwnd, &rc)
	return int32(rc.Right), int32(rc.Bottom)
}

// AddWidget adds a widget to the window and reparents its HWND into this window.
func (w *Window) AddWidget(widget Widget) {
	if widget == nil || w == nil {
		return
	}
	if child := widget.Handle(); child != 0 && w.hwnd != 0 {
		win32.SetParent(child, w.hwnd)
		// After reparent from host popup, force show + z-order under parent.
		win32.ShowWindow(child, win32.SW_SHOW)
		win32.SetWindowPos(child, 0, 0, 0, 0, 0,
			win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOZORDER|win32.SWP_FRAMECHANGED|win32.SWP_SHOWWINDOW)
		// Re-apply stored bounds so position is relative to the new parent.
		b := widget.Bounds()
		if b.Width > 0 && b.Height > 0 {
			widget.SetBounds(b)
		}
		widget.Invalidate()
	}
	w.widgets = append(w.widgets, widget)
	// Re-run layout so vbox/hbox takes effect without requiring a resize.
	if w.layout != nil {
		w.doLayout()
	}
}

// CW_USEDEFAULT constant
const CW_USEDEFAULT = 0x80000000


// Destroy implements Widget.
func (w *Window) Destroy() {
	if w.hwnd != 0 {
		win32.DestroyWindow(w.hwnd)
		w.hwnd = 0
	}
}
