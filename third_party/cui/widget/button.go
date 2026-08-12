package widget

import (
	"cui/core"
	"cui/win32"
)

// ButtonStyle represents button visual style
type ButtonStyle int

const (
	ButtonStyleNormal   ButtonStyle = 0
	ButtonStyleFlat     ButtonStyle = 1
	ButtonStyleOutlined ButtonStyle = 2
)

// Button is a push button widget
type Button struct {
	core.BaseWidget
	text        string
	style       ButtonStyle
	cornerRadius int32
}

// NewButton creates a new button widget
func NewButton(text string) *Button {
	app := core.GetApp()
	b := &Button{
		text:         text,
		style:        ButtonStyleNormal,
		cornerRadius: 4,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE,
		0, 0, 120, 32,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	b.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	b.BaseWidget.SetOnPaint(b.paint)
	// Re-bind embedded BaseWidget so paint/click hit the live object (not a discarded copy).
	core.BindBaseWidget(&b.BaseWidget)
	return b
}

// SetText sets the button text
func (b *Button) SetText(text string) {
	b.text = text
	b.Invalidate()
}

// Text returns the button text
func (b *Button) Text() string {
	return b.text
}

// SetCornerRadius sets the corner radius
func (b *Button) SetCornerRadius(r int32) {
	b.cornerRadius = r
	b.Invalidate()
}

// SetStyle sets the button style
func (b *Button) SetStyle(s ButtonStyle) {
	b.style = s
	b.Invalidate()
}

// paint draws the button
func (b *Button) paint(canvas *core.Canvas) {
	w, h := canvas.Size()
	rc := core.Rect{X: 0, Y: 0, Width: w, Height: h}

	font := b.BaseWidget.Font()
	if font == nil {
		font = core.DefaultFont()
	}

	// Determine colors based on state
	var bgColor, borderColor, textColor core.Color
	switch {
	case !b.BaseWidget.Enabled():
		bgColor = core.ColorDisabled
		borderColor = core.ColorBorder
		textColor = core.ColorDisabledText
	case b.BaseWidget.IsPressed():
		bgColor = core.ColorPressed
		borderColor = core.ColorFocused
		textColor = core.ColorBlack
	case b.BaseWidget.IsHovering():
		bgColor = core.ColorHover
		borderColor = core.ColorFocused
		textColor = core.ColorBlack
	default:
		bgColor = core.ColorWhite
		borderColor = core.ColorBorder
		textColor = core.ColorBlack
	}

	// Draw background
	if b.cornerRadius > 0 {
		canvas.FillRoundRect(0, 0, w, h, b.cornerRadius, b.cornerRadius, bgColor, borderColor, 1)
	} else {
		canvas.FillRect(0, 0, w, h, bgColor)
		canvas.DrawRect(0, 0, w, h, borderColor, 1)
	}

	// Draw focus ring
	if b.BaseWidget.IsFocused() {
		canvas.DrawRect(2, 2, w-4, h-4, core.ColorFocused, 1)
	}

	// Draw text centered
	canvas.DrawText(b.text, rc, textColor, font, win32.DT_CENTER|win32.DT_VCENTER|win32.DT_SINGLELINE)
}
