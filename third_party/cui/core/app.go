package core

import (
	"cui/win32"
	"sync"
	"syscall"
	"unsafe"
)

// App is the application singleton
type App struct {
	instance   win32.HMODULE
	mainWindow *Window
	windows    map[win32.HWND]*Window
	mu         sync.Mutex
	running    bool
	className  string
	// widgetHost is a hidden popup used as temporary parent for WS_CHILD
	// controls created before they are attached to a real window.
	// Win32 rejects CreateWindowEx(WS_CHILD, parent=0).
	widgetHost win32.HWND
	// uiThread is the thread id that called Run(); used by Quit from other threads.
	uiThread uint32
}

var (
	appInstance *App
	appOnce     sync.Once
)

// NewApp creates or returns the application singleton
func NewApp() *App {
	appOnce.Do(func() {
		DPIInit()
		win32.InitGdiplus()

		// Initialize common controls
		initICC := win32.INITCOMMONCONTROLSEX{
			DwSize: 8,
			DwICC:  win32.ICC_STANDARD_CLASSES | win32.ICC_WIN95_CLASSES,
		}
		win32.InitCommonControlsEx(&initICC)

		hInst := win32.GetModuleHandle(nil)

		appInstance = &App{
			instance:  hInst,
			windows:   make(map[win32.HWND]*Window),
			className: "CUIWindow",
		}

		appInstance.registerWindowClass()
	})

	return appInstance
}

// Instance returns the application module handle
func (a *App) Instance() win32.HMODULE {
	return a.instance
}

// registerWindowClass registers the main window class
func (a *App) registerWindowClass() {
	className := win32.UTF16PtrFromString(a.className)

	wc := win32.WNDCLASSEX{
		CbSize:        win32.UINT(unsafe.Sizeof(win32.WNDCLASSEX{})),
		Style:         win32.CS_HREDRAW | win32.CS_VREDRAW | win32.CS_DBLCLKS,
		LpfnWndProc:   wndProcCallback,
		HInstance:     a.instance,
		HCursor:       win32.LoadCursor(0, win32.IDC_ARROW),
		HbrBackground: win32.HBRUSH(win32.GetStockObject(win32.WHITE_BRUSH)),
		LpszClassName: className,
	}
	win32.RegisterClassEx(&wc)

	// Register a custom widget class for self-drawn controls
	widgetClassName := win32.UTF16PtrFromString("CUIWidget")
	wc2 := win32.WNDCLASSEX{
		CbSize:        win32.UINT(unsafe.Sizeof(win32.WNDCLASSEX{})),
		Style:         win32.CS_HREDRAW | win32.CS_VREDRAW | win32.CS_DBLCLKS | win32.CS_PARENTDC,
		LpfnWndProc:   wndProcCallback,
		HInstance:     a.instance,
		HCursor:       win32.LoadCursor(0, win32.IDC_ARROW),
		HbrBackground: 0, // no background, we paint ourselves
		LpszClassName: widgetClassName,
	}
	win32.RegisterClassEx(&wc2)
}

// addWindow registers a window with the app
func (a *App) addWindow(w *Window) {
	a.mu.Lock()
	a.windows[w.Handle()] = w
	a.mu.Unlock()
}

// removeWindow unregisters a window
func (a *App) removeWindow(hwnd win32.HWND) {
	a.mu.Lock()
	delete(a.windows, hwnd)
	a.mu.Unlock()
}

// getWindow retrieves a window by HWND
func (a *App) getWindow(hwnd win32.HWND) *Window {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[hwnd]
}

// NewWindow creates a new top-level window
func (a *App) NewWindow(title string, width, height int32) *Window {
	w := newWindow(a, title, width, height)
	a.addWindow(w)
	return w
}

// Run starts the message loop. This blocks until the application exits.
func (a *App) Run() {
	a.uiThread = win32.GetCurrentThreadId()
	a.running = true
	var msg win32.MSG

	for a.running {
		// Use PeekMessage + WaitMessage for better idle behavior
		if win32.PeekMessage(&msg, 0, 0, 0, 1) != 0 { // PM_REMOVE=1
			if msg.Message == win32.WM_QUIT {
				a.running = false
				break
			}
			win32.TranslateMessage(&msg)
			win32.DispatchMessage(&msg)
		} else {
			win32.WaitMessage()
		}
	}
}

// Quit exits the application. Safe to call from any thread: WM_QUIT is posted
// to the UI thread that is running App.Run (PostQuitMessage alone only affects
// the calling thread and can leave the message loop stuck in WaitMessage).
func (a *App) Quit() {
	if a == nil {
		return
	}
	a.running = false
	tid := a.uiThread
	if tid != 0 {
		win32.PostThreadMessage(tid, win32.WM_QUIT, 0, 0)
		return
	}
	win32.PostQuitMessage(0)
}

// MainClassName returns the registered window class name
func (a *App) MainClassName() string {
	return a.className
}

// WidgetClassName returns the registered widget class name
func (a *App) WidgetClassName() string {
	return "CUIWidget"
}

// WidgetHostHWND returns a hidden window that can act as temporary parent
// for WS_CHILD CreateWindowEx calls. Callers should SetParent to the real
// window when attaching the control.
func (a *App) WidgetHostHWND() win32.HWND {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.widgetHost != 0 {
		return a.widgetHost
	}
	// Hidden zero-size popup — valid parent for child HWNDs before reparent.
	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(a.className),
		win32.UTF16PtrFromString("CUIWidgetHost"),
		win32.WS_POPUP,
		0, 0, 1, 1,
		0, 0, a.instance, nil,
	)
	a.widgetHost = hwnd
	return hwnd
}

// ChildParent returns parent if non-zero, otherwise the widget host HWND.
func (a *App) ChildParent(parent win32.HWND) win32.HWND {
	if parent != 0 {
		return parent
	}
	return a.WidgetHostHWND()
}

// MessageLoop processes all pending messages
func (a *App) MessageLoop() {
	var msg win32.MSG
	for win32.PeekMessage(&msg, 0, 0, 0, 1) != 0 { // PM_REMOVE=1
		if msg.Message == win32.WM_QUIT {
			a.running = false
			return
		}
		win32.TranslateMessage(&msg)
		win32.DispatchMessage(&msg)
	}
}

// ModalLoop runs a modal message loop (blocks until flag is set to false)
func (a *App) ModalLoop(shouldStop *bool) {
	var msg win32.MSG
	for !*shouldStop {
		ret := win32.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		win32.TranslateMessage(&msg)
		win32.DispatchMessage(&msg)
	}
}

// GetApp returns the global app instance
func GetApp() *App {
	return appInstance
}

// Ensure sync is imported (used by dispatcher.go's syncMap2)
var _ sync.Mutex

// Ensure syscall is used
var _ = syscall.NewCallback
var _ unsafe.Pointer
