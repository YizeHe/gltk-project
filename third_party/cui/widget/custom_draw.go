package widget

import (
	"cui/core"
	"cui/win32"
)

// CustomDrawWidget is a base for fully custom-drawn controls (test 16)
type CustomDrawWidget struct {
	core.BaseWidget
	bgColor      core.Color
	borderColor  core.Color
	gradientFrom core.Color
	gradientTo   core.Color
	cornerRadius int32
	borderWidth  int32
	useGradient  bool
}

// NewCustomDrawWidget creates a new custom-drawn widget
func NewCustomDrawWidget() *CustomDrawWidget {
	app := core.GetApp()
	cd := &CustomDrawWidget{
		bgColor:      core.ColorWhite,
		borderColor:  core.ColorBorder,
		gradientFrom: core.NewRGB(200, 220, 240),
		gradientTo:   core.NewRGB(100, 150, 200),
		cornerRadius: 8,
		borderWidth:  1,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE,
		0, 0, 200, 100,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	cd.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	core.BindBaseWidget(&cd.BaseWidget)
	cd.BaseWidget.SetOnPaint(cd.paint)
	return cd
}

// SetBackgroundColor sets the background color (used when not using gradient)
func (cd *CustomDrawWidget) SetBackgroundColor(c core.Color) {
	cd.bgColor = c
	cd.Invalidate()
}

// SetBorderColor sets the border color
func (cd *CustomDrawWidget) SetBorderColor(c core.Color) {
	cd.borderColor = c
	cd.Invalidate()
}

// SetGradient sets the gradient colors (enables gradient mode)
func (cd *CustomDrawWidget) SetGradient(from, to core.Color) {
	cd.gradientFrom = from
	cd.gradientTo = to
	cd.useGradient = true
	cd.Invalidate()
}

// SetCornerRadius sets the corner radius for rounded rectangles
func (cd *CustomDrawWidget) SetCornerRadius(r int32) {
	cd.cornerRadius = r
	cd.Invalidate()
}

// SetBorderWidth sets the border width
func (cd *CustomDrawWidget) SetBorderWidth(w int32) {
	cd.borderWidth = w
	cd.Invalidate()
}

func (cd *CustomDrawWidget) paint(canvas *core.Canvas) {
	w, h := canvas.Size()
	saved := canvas.Save()
	defer canvas.Restore(saved)

	// Draw background
	if cd.useGradient {
		canvas.GradientFillV(0, 0, w, h, cd.gradientFrom, cd.gradientTo)
	} else if cd.cornerRadius > 0 {
		canvas.FillRoundRect(0, 0, w, h, cd.cornerRadius, cd.cornerRadius, cd.bgColor, cd.borderColor, cd.borderWidth)
	} else {
		canvas.FillRect(0, 0, w, h, cd.bgColor)
		if cd.borderWidth > 0 {
			canvas.DrawRect(0, 0, w, h, cd.borderColor, cd.borderWidth)
		}
	}

	// Draw border on top of gradient
	if cd.useGradient && cd.cornerRadius > 0 {
		canvas.DrawRoundRect(0, 0, w, h, cd.cornerRadius, cd.cornerRadius, cd.borderColor, cd.borderWidth)
	}
}
