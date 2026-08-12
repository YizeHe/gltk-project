package layout

import "cui/core"

// Layout interface for widget layouts
type Layout interface {
	SetPadding(p int32)
	SetSpacing(s int32)
	LayoutChildren(containerBounds core.Rect, widgets []core.Widget)
}

// BaseLayout provides common layout functionality
type BaseLayout struct {
	padding int32
	spacing int32
}

// SetPadding sets the layout padding
func (l *BaseLayout) SetPadding(p int32) {
	l.padding = p
}

// SetSpacing sets the spacing between widgets
func (l *BaseLayout) SetSpacing(s int32) {
	l.spacing = s
}
