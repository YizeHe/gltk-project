package win32

import (
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGlobalAlloc        = kernel32.NewProc("GlobalAlloc")
	procGlobalFree         = kernel32.NewProc("GlobalFree")
	procGlobalLock         = kernel32.NewProc("GlobalLock")
	procGlobalUnlock       = kernel32.NewProc("GlobalUnlock")
	procGetLastError       = kernel32.NewProc("GetLastError")
	procGetCurrentProcessId = kernel32.NewProc("GetCurrentProcessId")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	procOutputDebugStringW = kernel32.NewProc("OutputDebugStringW")
	procGetTickCount       = kernel32.NewProc("GetTickCount")
	procQueryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")
)

// GetModuleHandle retrieves a module handle for the specified module
func GetModuleHandle(moduleName *uint16) HMODULE {
	ret, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(moduleName)))
	return HMODULE(ret)
}

// GlobalAlloc allocates the specified number of bytes from the heap
func GlobalAlloc(flags UINT, bytes uintptr) HGLOBAL {
	ret, _, _ := procGlobalAlloc.Call(uintptr(flags), bytes)
	return HGLOBAL(ret)
}

// GlobalFree frees the specified global memory object
func GlobalFree(hMem HGLOBAL) HGLOBAL {
	ret, _, _ := procGlobalFree.Call(uintptr(hMem))
	return HGLOBAL(ret)
}

// GlobalLock locks a global memory object
func GlobalLock(hMem HGLOBAL) unsafe.Pointer {
	ret, _, _ := procGlobalLock.Call(uintptr(hMem))
	return unsafe.Pointer(ret)
}

// GlobalUnlock decrements the lock count
func GlobalUnlock(hMem HGLOBAL) BOOL {
	ret, _, _ := procGlobalUnlock.Call(uintptr(hMem))
	return BOOL(ret)
}

// GetLastError retrieves the calling thread's last-error code
func GetLastError() DWORD {
	ret, _, _ := procGetLastError.Call()
	return DWORD(ret)
}

// GetTickCount retrieves the number of milliseconds since the system was started
func GetTickCount() DWORD {
	ret, _, _ := procGetTickCount.Call()
	return DWORD(ret)
}

// GetCurrentThreadId returns the calling thread's thread identifier.
func GetCurrentThreadId() uint32 {
	ret, _, _ := procGetCurrentThreadId.Call()
	return uint32(ret)
}

// QueryPerformanceCounter retrieves the current value of the performance counter
func QueryPerformanceCounter(count *int64) BOOL {
	ret, _, _ := procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(count)))
	return BOOL(ret)
}

// QueryPerformanceFrequency retrieves the frequency of the performance counter
func QueryPerformanceFrequency(freq *int64) BOOL {
	ret, _, _ := procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(freq)))
	return BOOL(ret)
}
