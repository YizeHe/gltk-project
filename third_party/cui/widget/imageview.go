package widget

import (
	"cui/core"
	"cui/win32"
)

// ImageFitMode represents how the image fits in the view
type ImageFitMode int

const (
	ImageFitStretch ImageFitMode = iota // Stretch to fill
	ImageFitCenter                      // Center with original size
	ImageFitContain                     // Scale to fit, maintain aspect ratio
)

// ImageView is an image display widget
type ImageView struct {
	core.BaseWidget
	bitmap    *core.Bitmap
	fitMode   ImageFitMode
	bgColor   core.Color
}

// NewImageView creates a new image view widget
func NewImageView() *ImageView {
	app := core.GetApp()
	iv := &ImageView{
		fitMode: ImageFitContain,
		bgColor: core.ColorWhite,
	}

	hwnd := win32.CreateWindowEx(
		0,
		win32.UTF16PtrFromString(app.WidgetClassName()),
		nil,
		win32.WS_CHILD|win32.WS_VISIBLE,
		0, 0, 200, 200,
		app.ChildParent(0), 0, app.Instance(), nil,
	)
	if hwnd == 0 {
		return nil
	}

	iv.BaseWidget = *core.NewBaseWidgetFromHWND(hwnd)
	core.BindBaseWidget(&iv.BaseWidget)
	iv.BaseWidget.SetOnPaint(iv.paint)
	return iv
}

// LoadImage loads an image from a file path (supports BMP, PNG, JPG via GDI+)
func (iv *ImageView) LoadImage(path string) error {
	bmp, err := core.LoadBitmapFromFile(path)
	if err != nil {
		return err
	}
	iv.bitmap = bmp
	iv.Invalidate()
	return nil
}

// SetBitmap sets the bitmap directly
func (iv *ImageView) SetBitmap(bmp *core.Bitmap) {
	iv.bitmap = bmp
	iv.Invalidate()
}

// SetFitMode sets how the image fits in the view
func (iv *ImageView) SetFitMode(mode ImageFitMode) {
	iv.fitMode = mode
	iv.Invalidate()
}

// SetBackgroundColor sets the background color
func (iv *ImageView) SetBackgroundColor(c core.Color) {
	iv.bgColor = c
	iv.Invalidate()
}

// Clear clears the image
func (iv *ImageView) Clear() {
	iv.bitmap = nil
	iv.Invalidate()
}

func (iv *ImageView) paint(canvas *core.Canvas) {
	w, h := canvas.Size()

	// Fill background
	canvas.FillRect(0, 0, w, h, iv.bgColor)

	if iv.bitmap == nil || iv.bitmap.Handle == 0 {
		return
	}

	switch iv.fitMode {
	case ImageFitStretch:
		canvas.DrawBitmapStretched(iv.bitmap, 0, 0, w, h)
	case ImageFitCenter:
		// Center with original size, clip if necessary
		x := (w - iv.bitmap.Width) / 2
		y := (h - iv.bitmap.Height) / 2
		canvas.DrawBitmap(iv.bitmap, x, y)
	case ImageFitContain:
		canvas.DrawBitmapFit(iv.bitmap, 0, 0, w, h)
	}
}
