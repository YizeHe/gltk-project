package layout

import "cui/core"

// GridCell represents a widget's position in a grid
type GridCell struct {
	Widget  core.Widget
	Row     int32
	Col     int32
	RowSpan int32
	ColSpan int32
	HAlign  string
	VAlign  string
}

// GridLayout is a grid-based layout
type GridLayout struct {
	BaseLayout
	rows  int32
	cols  int32
	cells []GridCell
}

// NewGridLayout creates a new grid layout
func NewGridLayout(rows, cols int32) *GridLayout {
	return &GridLayout{
		rows: rows,
		cols: cols,
	}
}

// AddWidgetAt adds a widget at the specified grid position
func (l *GridLayout) AddWidgetAt(w core.Widget, row, col int32) {
	l.AddWidgetSpanAt(w, row, col, 1, 1)
}

// AddWidgetSpanAt adds a widget with row/column span
func (l *GridLayout) AddWidgetSpanAt(w core.Widget, row, col, rowspan, colspan int32) {
	l.cells = append(l.cells, GridCell{
		Widget:  w,
		Row:     row,
		Col:     col,
		RowSpan: rowspan,
		ColSpan: colspan,
		HAlign:  "center",
		VAlign:  "center",
	})
}

// LayoutChildren arranges widgets in a grid
func (l *GridLayout) LayoutChildren(containerBounds core.Rect, widgets []core.Widget) {
	if len(l.cells) == 0 || l.rows == 0 || l.cols == 0 {
		return
	}

	x0 := containerBounds.X + l.padding
	y0 := containerBounds.Y + l.padding
	availableWidth := containerBounds.Width - 2*l.padding
	availableHeight := containerBounds.Height - 2*l.padding

	totalColSpacing := l.spacing * (l.cols - 1)
	totalRowSpacing := l.spacing * (l.rows - 1)
	cellWidth := (availableWidth - totalColSpacing) / l.cols
	cellHeight := (availableHeight - totalRowSpacing) / l.rows

	for _, cell := range l.cells {
		widgetWidth := cellWidth*cell.ColSpan + l.spacing*(cell.ColSpan-1)
		widgetHeight := cellHeight*cell.RowSpan + l.spacing*(cell.RowSpan-1)

		cellX := x0 + cell.Col*(cellWidth+l.spacing)
		cellY := y0 + cell.Row*(cellHeight+l.spacing)

		cell.Widget.SetBounds(core.Rect{
			X:      cellX,
			Y:      cellY,
			Width:  widgetWidth,
			Height: widgetHeight,
		})
	}
}
