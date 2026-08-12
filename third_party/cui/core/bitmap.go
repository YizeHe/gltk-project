package core

import (
	"cui/win32"
	"unsafe"
)

// Bitmap represents a GDI bitmap
type Bitmap struct {
	Handle win32.HBITMAP
	Width  int32
	Height int32
}

// LoadBitmapFromFile loads a BMP file from disk
func LoadBitmapFromFile(path string) (*Bitmap, error) {
	hdc := win32.GetDC(0)
	if hdc == 0 {
		return nil, ErrCreateFailed
	}
	defer win32.ReleaseDC(0, hdc)

	// Use GDI+ to load image (supports BMP, PNG, JPG, etc.)
	hBitmap, w, h, err := win32.LoadImageAsHBITMAP(path)
	if err != nil {
		return nil, err
	}

	TrackGDI(win32.HGDIOBJ(hBitmap), 0, "bitmap")
	return &Bitmap{Handle: hBitmap, Width: w, Height: h}, nil
}

// LoadBitmapFromResource loads a bitmap from a Win32 resource
func LoadBitmapFromResource(hInst win32.HMODULE, name *uint16) *Bitmap {
	// This would use LoadBitmap from user32 - placeholder
	return nil
}

// CreateBitmap creates a compatible bitmap of the given size
func CreateBitmap(hdc win32.HDC, width, height int32) *Bitmap {
	h := win32.CreateCompatibleBitmap(hdc, width, height)
	if h == 0 {
		return nil
	}
	TrackGDI(win32.HGDIOBJ(h), 0, "bitmap")
	return &Bitmap{Handle: h, Width: width, Height: height}
}

// CreateDIBitmap creates a DIB section bitmap
func CreateDIBitmap(hdc win32.HDC, width, height int32) (*Bitmap, unsafe.Pointer) {
	bi := win32.BITMAPINFO{
		BmiHeader: win32.BITMAPINFOHEADER{
			BiSize:      uint32(unsafe.Sizeof(win32.BITMAPINFOHEADER{})),
			BiWidth:     width,
			BiHeight:    -height, // top-down
			BiPlanes:    1,
			BiBitCount:  32,
			BiCompression: 0, // BI_RGB
		},
	}
	var bits unsafe.Pointer
	h := win32.CreateDIBSection(hdc, &bi, 0, &bits, 0, 0)
	if h == 0 {
		return nil, nil
	}
	TrackGDI(win32.HGDIOBJ(h), 0, "dib")
	return &Bitmap{Handle: h, Width: width, Height: height}, bits
}

// Dispose releases the bitmap resource
func (b *Bitmap) Dispose() {
	if b != nil && b.Handle != 0 {
		win32.DeleteObject(win32.HGDIOBJ(b.Handle))
		b.Handle = 0
	}
}

// ErrCreateFailed is returned when resource creation fails
var ErrCreateFailed = &CUIError{"resource creation failed"}

type CUIError struct {
	msg string
}

func (e *CUIError) Error() string {
	return e.msg
}
