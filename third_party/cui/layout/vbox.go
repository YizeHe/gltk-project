package layout

import "cui/core"

// VBoxLayout is a vertical box layout
type VBoxLayout struct {
	BaseLayout
	fixedHeights map[core.Widget]int32
	autoStretch  bool
}

// NewVBoxLayout creates a new vertical box layout
func NewVBoxLayout() *VBoxLayout {
	return &VBoxLayout{
		fixedHeights: make(map[core.Widget]int32),
		autoStretch:  true,
	}
}

// NewVBox creates a vertical layout with padding and spacing
func NewVBox(padding, spacing int32) *VBoxLayout {
	return &VBoxLayout{
		BaseLayout:   BaseLayout{padding: padding, spacing: spacing},
		fixedHeights: make(map[core.Widget]int32),
		autoStretch:  true,
	}
}

// SetFixedHeight sets a fixed height for a widget in the layout
func (l *VBoxLayout) SetFixedHeight(w core.Widget, height int32) {
	l.fixedHeights[w] = height
}

// SetAutoStretch sets whether widgets stretch to fill width
func (l *VBoxLayout) SetAutoStretch(stretch bool) {
	l.autoStretch = stretch
}

// LayoutChildren arranges widgets vertically
func (l *VBoxLayout) LayoutChildren(containerBounds core.Rect, widgets []core.Widget) {
	if len(widgets) == 0 {
		return
	}

	x := containerBounds.X + l.padding
	y := containerBounds.Y + l.padding
	availableWidth := containerBounds.Width - 2*l.padding
	availableHeight := containerBounds.Height - 2*l.padding

	// Calculate total fixed height and count auto-height widgets
	var totalFixedHeight int32
	autoCount := int32(0)
	for _, w := range widgets {
		if fh, ok := l.fixedHeights[w]; ok {
			totalFixedHeight += fh
		} else {
			autoCount++
		}
	}

	totalSpacing := l.spacing * int32(len(widgets)-1)
	remainingHeight := availableHeight - totalFixedHeight - totalSpacing
	autoHeight := int32(0)
	if autoCount > 0 {
		autoHeight = remainingHeight / autoCount
		if autoHeight < 0 {
			autoHeight = 0
		}
	}

	for _, w := range widgets {
		h := autoHeight
		if fh, ok := l.fixedHeights[w]; ok {
			h = fh
		}

		width := availableWidth
		if !l.autoStretch {
			width = w.Bounds().Width
		}

		bounds := core.Rect{
			X:      x,
			Y:      y,
			Width:  width,
			Height: h,
		}
		w.SetBounds(bounds)
		y += h + l.spacing
	}
}

// MinSize calculates the minimum size needed
func (l *VBoxLayout) MinSize(widgets []core.Widget) (int32, int32) {
	if len(widgets) == 0 {
		return l.padding * 2, l.padding * 2
	}

	var maxW int32
	totalH := l.padding * 2
	for i, w := range widgets {
		b := w.Bounds()
		if b.Width > maxW {
			maxW = b.Width
		}
		totalH += b.Height
		if i > 0 {
			totalH += l.spacing
		}
	}
	return maxW + 2*l.padding, totalH
}
