//go:build windows && !ci

package platforminput

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var dropUser32 = windows.NewLazySystemDLL("user32.dll")

func DropPosition(handle uintptr) (float64, float64, bool) {
	if handle == 0 {
		return 0, 0, false
	}
	var point struct{ X, Y int32 }
	var rect struct{ Left, Top, Right, Bottom int32 }
	if ok, _, _ := dropUser32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point))); ok == 0 {
		return 0, 0, false
	}
	if ok, _, _ := dropUser32.NewProc("ScreenToClient").Call(handle, uintptr(unsafe.Pointer(&point))); ok == 0 {
		return 0, 0, false
	}
	if ok, _, _ := dropUser32.NewProc("GetClientRect").Call(handle, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return 0, 0, false
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return 0, 0, false
	}
	return float64(point.X-rect.Left) / float64(rect.Right-rect.Left), float64(point.Y-rect.Top) / float64(rect.Bottom-rect.Top), true
}
