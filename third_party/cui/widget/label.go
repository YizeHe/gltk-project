package widget

import (
	"cui/core"
	"cui/win32"
)

// Label is a static text widget
type Label struct {
	core.BaseWidget
	text      string
	textColor core.Color
	bgColor   core.Color
	align     uint32
	vAlign    uint32
	wordWrap  bool
	opacity   byte // 0-255
}

// NewLabel creates a new label widget
func NewLabel(text string) *Label {
	app := core.GetApp()
	l := &Label{
		text:      text,
		textColor: core.ColorBlack,
		bgColor:   core.ColorTransparent,
		align:     win32.DT_LEFT,
		vAlign:    win32.DT_VCENTER,
		wordWrap:  false,
		opacity:   255,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE,
		0, 0, 100, 25,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	l.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	l.BaseWidget.SetOnPaint(l.paint)
	core.BindBaseWidget(&l.BaseWidget)
	return l
}

// SetText sets the label text
func (l *Label) SetText(text string) {
	l.text = text
	l.Invalidate()
}

// Text returns the label text
func (l *Label) Text() string {
	return l.text
}

// SetTextColor sets the text color
func (l *Label) SetTextColor(c core.Color) {
	l.textColor = c
	l.Invalidate()
}

// SetBackgroundColor sets the background color
func (l *Label) SetBackgroundColor(c core.Color) {
	l.bgColor = c
	l.Invalidate()
}

// SetAlign sets horizontal alignment: "left", "center", "right"
func (l *Label) SetAlign(align string) {
	switch align {
	case "center":
		l.align = win32.DT_CENTER
	case "right":
		l.align = win32.DT_RIGHT
	default:
		l.align = win32.DT_LEFT
	}
	l.Invalidate()
}

// SetVAlign sets vertical alignment: "top", "center", "bottom"
func (l *Label) SetVAlign(align string) {
	switch align {
	case "top":
		l.vAlign = win32.DT_TOP
	case "bottom":
		l.vAlign = win32.DT_BOTTOM
	default:
		l.vAlign = win32.DT_VCENTER
	}
	l.Invalidate()
}

// SetWordWrap enables or disables word wrapping
func (l *Label) SetWordWrap(wrap bool) {
	l.wordWrap = wrap
	l.Invalidate()
}

// SetOpacity sets the text opacity (0-255)
func (l *Label) SetOpacity(opacity byte) {
	l.opacity = opacity
	l.Invalidate()
}

// paint draws the label
func (l *Label) paint(canvas *core.Canvas) {
	bounds := l.BaseWidget.Bounds()
	bounds.X = 0
	bounds.Y = 0

	// Always fill background so stale pixels / black garbage never show through.
	// Transparent bgColor means use white (window-default) fill.
	bg := l.bgColor
	if bg.A == 0 {
		bg = core.ColorWhite
	}
	canvas.FillRect(0, 0, bounds.Width, bounds.Height, bg)

	// Draw text
	format := l.align | l.vAlign | win32.DT_SINGLELINE
	if l.wordWrap {
		format = l.align | l.vAlign | win32.DT_WORDBREAK
		format &^= win32.DT_SINGLELINE
	}

	font := l.BaseWidget.Font()
	if font == nil {
		font = core.DefaultFont()
	}

	canvas.DrawText(l.text, bounds, l.textColor, font, format)
}
