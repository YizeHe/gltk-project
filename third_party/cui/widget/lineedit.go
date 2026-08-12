package widget

import (
	"cui/core"
	"cui/win32"
	"syscall"
	"unsafe"
)

// LineEdit is a single-line text input widget
type LineEdit struct {
	core.BaseWidget
	placeholder string
	maxLength   int32
	numberOnly  bool
	readOnly    bool
}

// NewLineEdit creates a new single-line edit control
func NewLineEdit() *LineEdit {
	app := core.GetApp()
	le := &LineEdit{}

	hwnd := win32.CreateWindowEx(
		win32.WS_EX_CLIENTEDGE,
		win32.UTF16PtrFromString("Edit"),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE|win32.ES_AUTOHSCROLL,
		0, 0, 200, 24,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	le.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	font := core.DefaultFont()
	if font != nil {
		le.BaseWidget.SetFont(font)
	}
	core.BindBaseWidget(&le.BaseWidget)
	return le
}

// SetText sets the edit text
func (le *LineEdit) SetText(text string) {
	win32.SetWindowText(le.Handle(), win32.UTF16PtrFromString(text))
}

// Text returns the edit text
func (le *LineEdit) Text() string {
	length := win32.GetWindowTextLength(le.Handle())
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	win32.GetWindowText(le.Handle(), &buf[0], length+1)
	return syscall.UTF16ToString(buf)
}

// SetPlaceholder sets the placeholder text (hint text)
func (le *LineEdit) SetPlaceholder(text string) {
	le.placeholder = text
	win32.SendMessage(le.Handle(), win32.EM_SETCUEBANNER, 1, win32.LPARAM(uintptr(unsafe.Pointer(win32.UTF16PtrFromString(text)))))
}

// SetMaxLength sets the maximum number of characters
func (le *LineEdit) SetMaxLength(max int32) {
	le.maxLength = max
	win32.SendMessage(le.Handle(), win32.EM_SETLIMITTEXT, win32.WPARAM(max), 0)
}

// SetNumberOnly restricts input to numbers only
func (le *LineEdit) SetNumberOnly(only bool) {
	le.numberOnly = only
	if only {
		le.BaseWidget.SetOnKeyDown(le.filterNumeric)
	}
}

// SetReadOnly sets read-only mode
func (le *LineEdit) SetReadOnly(readonly bool) {
	le.readOnly = readonly
	win32.SendMessage(le.Handle(), win32.EM_SETREADONLY, boolToWParam(readonly), 0)
}

// SelectAll selects all text
func (le *LineEdit) SelectAll() {
	win32.SendMessage(le.Handle(), win32.EM_SETSEL, 0, win32.LPARAM(^uint32(0)))
}

// SetSelection selects a range of text
func (le *LineEdit) SetSelection(start, end int32) {
	win32.SendMessage(le.Handle(), win32.EM_SETSEL, win32.WPARAM(start), win32.LPARAM(end))
}

// Clear clears the text
func (le *LineEdit) Clear() {
	win32.SetWindowText(le.Handle(), nil)
}

func (le *LineEdit) filterNumeric(ke core.KeyEvent) {
	if ke.Key == win32.VK_BACK || ke.Key == win32.VK_DELETE ||
		ke.Key == win32.VK_LEFT || ke.Key == win32.VK_RIGHT ||
		ke.Key == win32.VK_HOME || ke.Key == win32.VK_END {
		return
	}
	if (ke.Key >= '0' && ke.Key <= '9') ||
		(ke.Key >= win32.VK_NUMPAD0 && ke.Key <= win32.VK_NUMPAD9) {
		return
	}
	if ke.Modifiers&core.KeyModControl != 0 {
		return
	}
}

func boolToWParam(b bool) win32.WPARAM {
	if b {
		return 1
	}
	return 0
}
