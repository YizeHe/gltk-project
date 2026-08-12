package widget

import (
	"cui/core"
	"cui/win32"
)

// Panel is a container widget that can hold child widgets
type Panel struct {
	core.BaseWidget
	bgColor  core.Color
	layout   core.Layout
	widgets  []core.Widget
}

// NewPanel creates a new panel widget
func NewPanel() *Panel {
	app := core.GetApp()
	p := &Panel{
		bgColor: core.ColorTransparent,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE|win32.WS_CLIPCHILDREN,
		0, 0, 200, 200,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	p.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	core.BindBaseWidget(&p.BaseWidget)
	p.BaseWidget.SetOnPaint(p.paint)
	return p
}

// SetBackgroundColor sets the panel background color
func (p *Panel) SetBackgroundColor(c core.Color) {
	p.bgColor = c
	p.Invalidate()
}

// SetLayout sets the panel layout
func (p *Panel) SetLayout(l core.Layout) {
	p.layout = l
}

// Layout returns the panel layout
func (p *Panel) Layout() core.Layout {
	return p.layout
}

// AddWidget adds a child widget to the panel
func (p *Panel) AddWidget(w core.Widget) {
	p.widgets = append(p.widgets, w)
}

// doLayout triggers a layout pass
func (p *Panel) doLayout() {
	if p.layout != nil && len(p.widgets) > 0 {
		bounds := p.BaseWidget.Bounds()
		bounds.X = 0
		bounds.Y = 0
		p.layout.LayoutChildren(bounds, p.widgets)
	}
}

func (p *Panel) paint(canvas *core.Canvas) {
	w, h := canvas.Size()
	if p.bgColor.A > 0 {
		canvas.FillRect(0, 0, w, h, p.bgColor)
	}
	p.doLayout()
}
