package widget

import (
	"cui/core"
	"cui/win32"
	"syscall"
	"unsafe"
)

// TextEdit is a multi-line text editor widget
type TextEdit struct {
	core.BaseWidget
	readOnly bool
}

// NewTextEdit creates a new multi-line text edit control
func NewTextEdit() *TextEdit {
	app := core.GetApp()
	te := &TextEdit{}

	hwnd := win32.CreateWindowEx(
		win32.WS_EX_CLIENTEDGE,
		win32.UTF16PtrFromString("Edit"),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE|win32.ES_MULTILINE|win32.ES_AUTOVSCROLL|win32.ES_WANTRETURN|win32.WS_VSCROLL,
		0, 0, 300, 200,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	te.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	font := core.DefaultFont()
	if font != nil {
		te.BaseWidget.SetFont(font)
	}
	core.BindBaseWidget(&te.BaseWidget)
	return te
}

// SetText sets the text content
func (te *TextEdit) SetText(text string) {
	win32.SetWindowText(te.Handle(), win32.UTF16PtrFromString(text))
}

// Text returns the text content
func (te *TextEdit) Text() string {
	length := win32.GetWindowTextLength(te.Handle())
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	win32.GetWindowText(te.Handle(), &buf[0], length+1)
	return syscall.UTF16ToString(buf)
}

// AppendText appends text to the end
func (te *TextEdit) AppendText(text string) {
	length := win32.GetWindowTextLength(te.Handle())
	win32.SendMessage(te.Handle(), win32.EM_SETSEL, win32.WPARAM(length), win32.LPARAM(length))
	win32.SendMessage(te.Handle(), win32.EM_REPLACESEL, 1, win32.LPARAM(uintptr(unsafe.Pointer(win32.UTF16PtrFromString(text)))))
}

// Clear clears all text
func (te *TextEdit) Clear() {
	win32.SetWindowText(te.Handle(), nil)
}

// SetReadOnly sets read-only mode
func (te *TextEdit) SetReadOnly(readonly bool) {
	te.readOnly = readonly
	win32.SendMessage(te.Handle(), win32.EM_SETREADONLY, boolToWParam(readonly), 0)
}

// SetCaretPos sets the caret/cursor position
func (te *TextEdit) SetCaretPos(pos int32) {
	win32.SendMessage(te.Handle(), win32.EM_SETSEL, win32.WPARAM(pos), win32.LPARAM(pos))
	win32.SendMessage(te.Handle(), win32.EM_SCROLLCARET, 0, 0)
}

// LineCount returns the number of lines
func (te *TextEdit) LineCount() int32 {
	return int32(win32.SendMessage(te.Handle(), win32.EM_GETLINECOUNT, 0, 0))
}

// ScrollToBottom scrolls to the bottom
func (te *TextEdit) ScrollToBottom() {
	lineCount := te.LineCount()
	win32.SendMessage(te.Handle(), win32.EM_LINESCROLL, 0, win32.LPARAM(lineCount))
}
