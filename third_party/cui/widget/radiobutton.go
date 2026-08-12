package widget

import (
	"cui/core"
	"cui/win32"
)

// RadioButton is a radio button widget
type RadioButton struct {
	core.BaseWidget
	text      string
	checked   bool
	textColor core.Color
	group     []*RadioButton
	userClick core.ClickCallback // separate from selectRadio (avoids recursion)
}

// NewRadioButton creates a new radio button widget
func NewRadioButton(text string) *RadioButton {
	app := core.GetApp()
	rb := &RadioButton{
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

	rb.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	core.BindBaseWidget(&rb.BaseWidget)
	rb.BaseWidget.SetOnPaint(rb.paint)
	rb.BaseWidget.SetOnClick(rb.selectRadio)
	return rb
}

// SetText sets the radio button text
func (rb *RadioButton) SetText(text string) {
	rb.text = text
	rb.Invalidate()
}

// Checked returns the checked state
func (rb *RadioButton) Checked() bool {
	return rb.checked
}

// SetChecked sets the checked state and unchecks siblings
func (rb *RadioButton) SetChecked(checked bool) {
	if checked {
		// Uncheck all siblings in the same group
		for _, sibling := range rb.group {
			if sibling != rb {
				sibling.checked = false
				sibling.Invalidate()
			}
		}
	}
	rb.checked = checked
	rb.Invalidate()
}

// SetTextColor sets the text color
func (rb *RadioButton) SetTextColor(c core.Color) {
	rb.textColor = c
	rb.Invalidate()
}

// SetGroup sets the radio button group
func (rb *RadioButton) SetGroup(group []*RadioButton) {
	rb.group = group
}

// SetOnClick sets the user click callback (fired after selection).
func (rb *RadioButton) SetOnClick(fn core.ClickCallback) {
	rb.userClick = fn
}

func (rb *RadioButton) selectRadio() {
	rb.SetChecked(true)
	if rb.userClick != nil {
		rb.userClick()
	}
}

func (rb *RadioButton) paint(canvas *core.Canvas) {
	w, h := canvas.Size()
	font := rb.BaseWidget.Font()
	if font == nil {
		font = core.DefaultFont()
	}

	// Draw radio circle
	circleSize := int32(16)
	circleY := (h - circleSize) / 2
	cx := int32(4 + circleSize/2)
	cy := circleY + circleSize/2

	// Draw circle background
	var circleBg, circleBorder core.Color
	switch {
	case !rb.BaseWidget.Enabled():
		circleBg = core.ColorDisabled
		circleBorder = core.ColorBorder
	case rb.BaseWidget.IsHovering():
		circleBg = core.ColorHover
		circleBorder = core.ColorFocused
	default:
		circleBg = core.ColorWhite
		circleBorder = core.ColorBorder
	}

	canvas.FillEllipse(4, circleY, circleSize, circleSize, circleBg, circleBorder, 1)

	// Draw filled circle if checked
	if rb.checked {
		innerSize := int32(8)
		innerX := cx - innerSize/2
		innerY := cy - innerSize/2
		canvas.FillEllipse(innerX, innerY, innerSize, innerSize, core.ColorFocused, core.ColorFocused, 0)
	}

	// Draw text
	textX := int32(4 + circleSize + 6)
	textRect := core.Rect{X: textX, Y: 0, Width: w - textX, Height: h}
	canvas.DrawText(rb.text, textRect, rb.textColor, font, win32.DT_LEFT|win32.DT_VCENTER|win32.DT_SINGLELINE)
}
