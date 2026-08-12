package menu

import (
	"cui/win32"
	"sync"
)

// Menu represents a Win32 menu
type Menu struct {
	handle    win32.HMENU
	items     []*MenuItem
	isPopup   bool
}

// MenuItem represents a menu item
type MenuItem struct {
	id       uint32
	text     string
	enabled  bool
	checked  bool
	separator bool
	submenu  *Menu
	callback func()
}

var (
	menuIDCounter uint32 = 2000
	menuCallbacks = make(map[uint32]func())
	menuCallbacksMu sync.Mutex
)

func nextMenuID() uint32 {
	menuIDCounter++
	return menuIDCounter
}

// NewMenu creates a new menu bar
func NewMenu() *Menu {
	h := win32.CreateMenu()
	return &Menu{handle: h, items: make([]*MenuItem, 0)}
}

// NewPopupMenu creates a new popup/context menu
func NewPopupMenu() *Menu {
	h := win32.CreatePopupMenu()
	return &Menu{handle: h, items: make([]*MenuItem, 0), isPopup: true}
}

// Handle returns the native menu handle
func (m *Menu) Handle() win32.HMENU {
	return m.handle
}

// AddItem adds a menu item with text and callback
func (m *Menu) AddItem(text string, callback func()) *MenuItem {
	id := nextMenuID()
	item := &MenuItem{
		id:       id,
		text:     text,
		enabled:  true,
		callback: callback,
	}

	win32.AppendMenu(m.handle, win32.MF_STRING, uintptr(id), win32.UTF16PtrFromString(text))

	menuCallbacksMu.Lock()
	menuCallbacks[id] = callback
	menuCallbacksMu.Unlock()

	m.items = append(m.items, item)
	return item
}

// AddSeparator adds a separator line
func (m *Menu) AddSeparator() *MenuItem {
	item := &MenuItem{separator: true, enabled: true}
	win32.AppendMenu(m.handle, win32.MF_SEPARATOR, 0, nil)
	m.items = append(m.items, item)
	return item
}

// AddSubMenu adds a submenu
func (m *Menu) AddSubMenu(text string, submenu *Menu) *MenuItem {
	item := &MenuItem{
		text:    text,
		enabled: true,
		submenu: submenu,
	}

	win32.AppendMenu(m.handle, win32.MF_POPUP|win32.MF_STRING, uintptr(submenu.handle), win32.UTF16PtrFromString(text))
	m.items = append(m.items, item)
	return item
}

// SetEnabled enables or disables a menu item by index
func (m *Menu) SetEnabled(index int, enabled bool) {
	if index >= 0 && index < len(m.items) {
		item := m.items[index]
		item.enabled = enabled
		if enabled {
			win32.EnableMenuItem(m.handle, win32.UINT(index), win32.MF_BYPOSITION|win32.MF_ENABLED)
		} else {
			win32.EnableMenuItem(m.handle, win32.UINT(index), win32.MF_BYPOSITION|win32.MF_GRAYED)
		}
	}
}

// SetChecked sets the checked state of a menu item by index
func (m *Menu) SetChecked(index int, checked bool) {
	if index >= 0 && index < len(m.items) {
		item := m.items[index]
		item.checked = checked
		if checked {
			win32.CheckMenuItem(m.handle, win32.UINT(index), win32.MF_BYPOSITION|win32.MF_CHECKED)
		} else {
			win32.CheckMenuItem(m.handle, win32.UINT(index), win32.MF_BYPOSITION|win32.MF_UNCHECKED)
		}
	}
}

// Show shows a popup menu at the given screen coordinates
func (m *Menu) Show(hwnd win32.HWND, x, y int32) {
	win32.SetForegroundWindow(hwnd)
	win32.TrackPopupMenu(m.handle, win32.TPM_LEFTALIGN|win32.TPM_RIGHTBUTTON, x, y, 0, hwnd, nil)
}

// Destroy destroys the menu
func (m *Menu) Destroy() {
	if m.handle != 0 {
		win32.DestroyMenu(m.handle)
		m.handle = 0
	}
}

// HandleMenuCommand processes a WM_COMMAND message for menu items
func HandleMenuCommand(id uint32) bool {
	menuCallbacksMu.Lock()
	callback, ok := menuCallbacks[id]
	menuCallbacksMu.Unlock()
	if ok && callback != nil {
		callback()
		return true
	}
	return false
}

// MenuBar represents a window's menu bar
type MenuBar struct {
	handle win32.HMENU
	items  []*Menu
}

// NewMenuBar creates a new menu bar
func NewMenuBar() *MenuBar {
	h := win32.CreateMenu()
	return &MenuBar{handle: h}
}

// AddMenu adds a top-level menu to the menu bar
func (mb *MenuBar) AddMenu(text string, menu *Menu) {
	win32.AppendMenu(mb.handle, win32.MF_POPUP|win32.MF_STRING, uintptr(menu.handle), win32.UTF16PtrFromString(text))
	mb.items = append(mb.items, menu)
}

// SetToWindow attaches the menu bar to a window
func (mb *MenuBar) SetToWindow(hwnd win32.HWND) {
	win32.SetMenu(hwnd, mb.handle)
}

// Handle returns the native menu bar handle
func (mb *MenuBar) Handle() win32.HMENU {
	return mb.handle
}
