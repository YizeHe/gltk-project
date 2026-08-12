package widget

import (
	"cui/core"
	"cui/win32"
)

// CheckBox is a checkbox widget
type CheckBox struct {
	core.BaseWidget
	text      string
	checked   bool
	textColor core.Color
	userClick core.ClickCallback // separate from internal toggle (avoids recursion)
}

// NewCheckBox creates a new checkbox widget
func NewCheckBox(text string) *CheckBox {
	app := core.GetApp()
	cb := &CheckBox{
		text:      text,
		textColor: core.ColorBlack,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE,
		0, 0, 150, 24,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	cb.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	cb.BaseWidget.SetOnPaint(cb.paint)
	// Internal toggle only — user handlers go through SetOnClick override.
	cb.BaseWidget.SetOnClick(cb.toggle)
	core.BindBaseWidget(&cb.BaseWidget)
	return cb
}

// SetOnClick sets the user click callback (fired after checked state toggles).
// Does not replace the internal toggle handler.
func (cb *CheckBox) SetOnClick(fn core.ClickCallback) {
	cb.userClick = fn
}

// SetText sets the checkbox text
func (cb *CheckBox) SetText(text string) {
	cb.text = text
	cb.Invalidate()
}

// Text returns the checkbox label text
func (cb *CheckBox) Text() string {
	return cb.text
}

// Checked returns the checked state
func (cb *CheckBox) Checked() bool {
	return cb.checked
}

// SetChecked sets the checked state
func (cb *CheckBox) SetChecked(checked bool) {
	cb.checked = checked
	cb.Invalidate()
}

// SetTextColor sets the text color
func (cb *CheckBox) SetTextColor(c core.Color) {
	cb.textColor = c
	cb.Invalidate()
}

func (cb *CheckBox) toggle() {
	cb.checked = !cb.checked
	cb.Invalidate()
	if cb.userClick != nil {
		cb.userClick()
	}
}

func (cb *CheckBox) paint(canvas *core.Canvas) {
	w, h := canvas.Size()
	font := cb.BaseWidget.Font()
	if font == nil {
		font = core.DefaultFont()
	}

	// Clear background so no black garbage shows through.
	canvas.FillRect(0, 0, w, h, core.ColorWhite)

	// Draw checkbox box
	boxSize := int32(16)
	boxY := (h - boxSize) / 2

	// Draw box background
	var boxBg, boxBorder core.Color
	switch {
	case !cb.BaseWidget.Enabled():
		boxBg = core.ColorDisabled
		boxBorder = core.ColorBorder
	case cb.BaseWidget.IsHovering():
		boxBg = core.ColorHover
		boxBorder = core.ColorFocused
	default:
		boxBg = core.ColorWhite
		boxBorder = core.ColorBorder
	}

	canvas.FillRect(4, boxY, boxSize, boxSize, boxBg)
	canvas.DrawRect(4, boxY, boxSize, boxSize, boxBorder, 1)

	// Draw check mark if checked
	if cb.checked {
		// Draw a simple checkmark using lines
		pen := core.NewSolidPen(2, core.ColorFocused)
		oldPen := win32.SelectObject(canvas.HDC, win32.HGDIOBJ(pen.Handle))
		// Checkmark: two line segments forming a "V"
		win32.MoveToEx(canvas.HDC, 7, boxY+8, nil)
		win32.LineTo(canvas.HDC, 10, boxY+boxSize-3)
		win32.LineTo(canvas.HDC, 4+boxSize-3, boxY+4)
		win32.SelectObject(canvas.HDC, oldPen)
		pen.Dispose()
	}

	// Draw text
	textX := int32(4 + boxSize + 6)
	textRect := core.Rect{X: textX, Y: 0, Width: w - textX, Height: h}
	canvas.DrawText(cb.text, textRect, cb.textColor, font, win32.DT_LEFT|win32.DT_VCENTER|win32.DT_SINGLELINE)
}
