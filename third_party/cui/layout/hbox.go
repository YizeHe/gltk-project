package layout

import "cui/core"

// HBoxLayout is a horizontal box layout
type HBoxLayout struct {
	BaseLayout
}

// NewHBoxLayout creates a new horizontal box layout
func NewHBoxLayout() *HBoxLayout {
	return &HBoxLayout{}
}

// NewHBox creates a horizontal layout with padding and spacing
func NewHBox(padding, spacing int32) *HBoxLayout {
	return &HBoxLayout{
		BaseLayout: BaseLayout{padding: padding, spacing: spacing},
	}
}

// LayoutChildren arranges widgets horizontally
func (l *HBoxLayout) LayoutChildren(containerBounds core.Rect, widgets []core.Widget) {
	if len(widgets) == 0 {
		return
	}

	x := containerBounds.X + l.padding
	y := containerBounds.Y + l.padding
	availableWidth := containerBounds.Width - 2*l.padding
	availableHeight := containerBounds.Height - 2*l.padding

	totalSpacing := l.spacing * int32(len(widgets)-1)
	widgetWidth := (availableWidth - totalSpacing) / int32(len(widgets))
	if widgetWidth < 0 {
		widgetWidth = 0
	}

	for _, w := range widgets {
		bounds := core.Rect{
			X:      x,
			Y:      y,
			Width:  widgetWidth,
			Height: availableHeight,
		}
		w.SetBounds(bounds)
		x += widgetWidth + l.spacing
	}
}

// MinSize calculates the minimum size needed
func (l *HBoxLayout) MinSize(widgets []core.Widget) (int32, int32) {
	if len(widgets) == 0 {
		return l.padding * 2, l.padding * 2
	}

	var maxH int32
	totalW := l.padding * 2
	for i, w := range widgets {
		b := w.Bounds()
		totalW += b.Width
		if b.Height > maxH {
			maxH = b.Height
		}
		if i > 0 {
			totalW += l.spacing
		}
	}
	return totalW, maxH + 2*l.padding
}
