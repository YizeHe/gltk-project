package core

import (
	"cui/win32"
	"sync"
)

// ResourceManager tracks GDI resources for automatic cleanup
type ResourceManager struct {
	mu        sync.Mutex
	resources []gdiResource
	byWindow  map[win32.HWND][]int // indices into resources by owner window
}

type gdiResource struct {
	handle win32.HGDIOBJ
	owner  win32.HWND // 0 = global, owned by window
	kind   string      // for debugging
}

var globalRM = &ResourceManager{
	byWindow: make(map[win32.HWND][]int),
}

// Track adds a GDI resource to be tracked
func (rm *ResourceManager) Track(handle win32.HGDIOBJ, owner win32.HWND, kind string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	idx := len(rm.resources)
	rm.resources = append(rm.resources, gdiResource{handle: handle, owner: owner, kind: kind})
	if owner != 0 {
		rm.byWindow[owner] = append(rm.byWindow[owner], idx)
	}
}

// ReleaseWindow releases all GDI resources owned by a window
func (rm *ResourceManager) ReleaseWindow(hwnd win32.HWND) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	indices := rm.byWindow[hwnd]
	for _, idx := range indices {
		res := &rm.resources[idx]
		if res.handle != 0 {
			win32.DeleteObject(res.handle)
			res.handle = 0
		}
	}
	delete(rm.byWindow, hwnd)
}

// ReleaseAll releases all tracked resources
func (rm *ResourceManager) ReleaseAll() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for i := range rm.resources {
		res := &rm.resources[i]
		if res.handle != 0 {
			win32.DeleteObject(res.handle)
			res.handle = 0
		}
	}
	rm.resources = nil
	rm.byWindow = make(map[win32.HWND][]int)
}

// TrackGDI adds a GDI resource to the global manager
func TrackGDI(handle win32.HGDIOBJ, owner win32.HWND, kind string) {
	globalRM.Track(handle, owner, kind)
}

// ReleaseWindowResources releases all GDI resources for a window
func ReleaseWindowResources(hwnd win32.HWND) {
	globalRM.ReleaseWindow(hwnd)
}
