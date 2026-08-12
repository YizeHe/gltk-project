package core

import (
	"cui/win32"
	"sync"
)

var (
	dpiOnce     sync.Once
	dpiScale    float64 = 1.0
	dpiAware   bool
	systemDPI   int32 = 96
)

// DPIInit initializes DPI awareness. Must be called before creating any windows.
func DPIInit() {
	dpiOnce.Do(func() {
		// Try Per-Monitor V2 first (Windows 10 1703+)
		ret := win32.SetProcessDpiAwarenessContext(win32.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
		if ret == 0 {
			// Fallback to system DPI aware
			win32.SetProcessDPIAware()
		}
		dpiAware = true
		systemDPI = int32(win32.GetDpiForSystem())
		if systemDPI == 0 {
			systemDPI = 96
		}
		dpiScale = float64(systemDPI) / 96.0
	})
}

// DPIScale returns the current DPI scale factor
func DPIScale() float64 {
	return dpiScale
}

// DPIForWindow returns the DPI for a specific window
func DPIForWindow(hwnd win32.HWND) int32 {
	dpi := win32.GetDpiForWindow(hwnd)
	if dpi == 0 {
		return systemDPI
	}
	return int32(dpi)
}

// DPIScaleForWindow returns the DPI scale factor for a specific window
func DPIScaleForWindow(hwnd win32.HWND) float64 {
	return float64(DPIForWindow(hwnd)) / 96.0
}

// DPIScaleValue scales a value by the DPI factor
func DPIScaleValue(value int32) int32 {
	return win32.MulDiv(value, int32(dpiScale*96), 96)
}

// DPIScaleValueForWindow scales a value by the DPI factor for a specific window
func DPIScaleValueForWindow(value int32, hwnd win32.HWND) int32 {
	scale := DPIScaleForWindow(hwnd)
	return int32(float64(value) * scale)
}

// DPIToPoints converts point size to pixel height for CreateFont
func DPIToPoints(points int32) int32 {
	return win32.MulDiv(points, systemDPI, 72)
}

// DPIToPointsForWindow converts point size to pixel height for a specific window
func DPIToPointsForWindow(points int32, hwnd win32.HWND) int32 {
	dpi := DPIForWindow(hwnd)
	return win32.MulDiv(points, dpi, 72)
}
