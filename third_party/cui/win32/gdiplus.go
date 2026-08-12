package win32

import (
	"syscall"
	"unsafe"
)

var (
	gdiplus = syscall.NewLazyDLL("gdiplus.dll")

	procGdiplusStartup                 = gdiplus.NewProc("GdiplusStartup")
	procGdiplusShutdown                = gdiplus.NewProc("GdiplusShutdown")
	procGdipCreateBitmapFromFile       = gdiplus.NewProc("GdipCreateBitmapFromFile")
	procGdipDisposeImage               = gdiplus.NewProc("GdipDisposeImage")
	procGdipCreateHBITMAPFromBitmap    = gdiplus.NewProc("GdipCreateHBITMAPFromBitmap")
	procGdipGetImageWidth              = gdiplus.NewProc("GdipGetImageWidth")
	procGdipGetImageHeight             = gdiplus.NewProc("GdipGetImageHeight")
	procGdipCreateBitmapFromHBITMAP    = gdiplus.NewProc("GdipCreateBitmapFromHBITMAP")
	procGdipCreateBitmapFromFileICW    = gdiplus.NewProc("GdipCreateBitmapFromFileICW")
	procGdipDrawImageRectI             = gdiplus.NewProc("GdipDrawImageRectI")
	procGdipCreateFromHDC              = gdiplus.NewProc("GdipCreateFromHDC")
	procGdipDeleteGraphics             = gdiplus.NewProc("GdipDeleteGraphics")
	procGdipSetInterpolationMode       = gdiplus.NewProc("GdipSetInterpolationMode")
)

// GdiplusStartupInput structure
type GdiplusStartupInputEx struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread BOOL
	SuppressExternalCodecs   BOOL
}

// GpStatus type
type GpStatus int32

const (
	Ok                     GpStatus = 0
	GenericError           GpStatus = 1
	InvalidParameter       GpStatus = 2
	OutOfMemory            GpStatus = 3
	ObjectBusy             GpStatus = 4
	InsufficientBuffer     GpStatus = 5
	NotImplemented         GpStatus = 6
	Win32Error             GpStatus = 7
	WrongState             GpStatus = 8
	Aborted                GpStatus = 9
	FileNotFound           GpStatus = 10
	ValueOverflow          GpStatus = 11
	AccessDenied           GpStatus = 12
	UnknownImageFormat     GpStatus = 13
	FontFamilyNotFound     GpStatus = 14
	FontStyleNotFound      GpStatus = 15
	NotTrueTypeFont        GpStatus = 16
	UnsupportedGdiplusVersion GpStatus = 17
	GdiplusNotInitialized  GpStatus = 18
	PropertyNotFound       GpStatus = 19
	PropertyNotSupported   GpStatus = 20
)

// Interpolation mode
const (
	InterpolationModeNearestNeighbor  = 5
	InterpolationModeBilinear         = 3
	InterpolationModeBicubic          = 4
	InterpolationModeHighQualityBicubic = 7
)

var gdiplusToken uintptr

// GdiplusStartup initializes GDI+
func GdiplusStartup(token *uintptr, input *GdiplusStartupInputEx, output unsafe.Pointer) GpStatus {
	ret, _, _ := procGdiplusStartup.Call(
		uintptr(unsafe.Pointer(token)),
		uintptr(unsafe.Pointer(input)),
		uintptr(output),
	)
	return GpStatus(ret)
}

// GdiplusShutdown shuts down GDI+
func GdiplusShutdown(token uintptr) {
	procGdiplusShutdown.Call(token)
}

// InitGdiplus initializes GDI+ and returns the token
func InitGdiplus() uintptr {
	if gdiplusToken != 0 {
		return gdiplusToken
	}
	input := GdiplusStartupInputEx{
		GdiplusVersion: 1,
	}
	GdiplusStartup(&gdiplusToken, &input, nil)
	return gdiplusToken
}

// ShutdownGdiplus shuts down GDI+
func ShutdownGdiplus() {
	if gdiplusToken != 0 {
		GdiplusShutdown(gdiplusToken)
		gdiplusToken = 0
	}
}

// GdipCreateBitmapFromFile creates a bitmap from a file (supports PNG, BMP, etc.)
func GdipCreateBitmapFromFile(filename *uint16, bitmap **GpBitmap) GpStatus {
	ret, _, _ := procGdipCreateBitmapFromFile.Call(
		uintptr(unsafe.Pointer(filename)),
		uintptr(unsafe.Pointer(bitmap)),
	)
	return GpStatus(ret)
}

// GdipDisposeImage releases an image
func GdipDisposeImage(image *GpBitmap) GpStatus {
	ret, _, _ := procGdipDisposeImage.Call(uintptr(unsafe.Pointer(image)))
	return GpStatus(ret)
}

// GdipCreateHBITMAPFromBitmap creates an HBITMAP from a GDI+ bitmap
func GdipCreateHBITMAPFromBitmap(bitmap *GpBitmap, hbmReturn *HBITMAP, background uint32) GpStatus {
	ret, _, _ := procGdipCreateHBITMAPFromBitmap.Call(
		uintptr(unsafe.Pointer(bitmap)),
		uintptr(unsafe.Pointer(hbmReturn)),
		uintptr(background),
	)
	return GpStatus(ret)
}

// GdipGetImageWidth gets the width of an image
func GdipGetImageWidth(image *GpBitmap, width *uint32) GpStatus {
	ret, _, _ := procGdipGetImageWidth.Call(
		uintptr(unsafe.Pointer(image)),
		uintptr(unsafe.Pointer(width)),
	)
	return GpStatus(ret)
}

// GdipGetImageHeight gets the height of an image
func GdipGetImageHeight(image *GpBitmap, height *uint32) GpStatus {
	ret, _, _ := procGdipGetImageHeight.Call(
		uintptr(unsafe.Pointer(image)),
		uintptr(unsafe.Pointer(height)),
	)
	return GpStatus(ret)
}

// LoadImageAsHBITMAP loads an image file (PNG/BMP/etc.) and returns an HBITMAP
func LoadImageAsHBITMAP(filename string) (HBITMAP, int32, int32, error) {
	InitGdiplus()

	wFilename := UTF16PtrFromString(filename)
	var bitmap *GpBitmap
	status := GdipCreateBitmapFromFile(wFilename, &bitmap)
	if status != Ok {
		return 0, 0, 0, syscall.Errno(status)
	}
	defer GdipDisposeImage(bitmap)

	var w, h uint32
	GdipGetImageWidth(bitmap, &w)
	GdipGetImageHeight(bitmap, &h)

	var hBitmap HBITMAP
	status = GdipCreateHBITMAPFromBitmap(bitmap, &hBitmap, 0xFF000000) // black background
	if status != Ok {
		return 0, 0, 0, syscall.Errno(status)
	}

	return hBitmap, int32(w), int32(h), nil
}
