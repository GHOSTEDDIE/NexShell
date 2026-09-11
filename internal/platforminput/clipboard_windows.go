//go:build windows && !ci

package platforminput

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

var clipboardUser = windows.NewLazySystemDLL("user32.dll")
var clipboardShell = windows.NewLazySystemDLL("shell32.dll")

func ClipboardFiles() ([]string, error) {
	has, _, _ := clipboardUser.NewProc("IsClipboardFormatAvailable").Call(15)
	if has == 0 {
		return nil, nil
	}
	opened, _, err := clipboardUser.NewProc("OpenClipboard").Call(0)
	if opened == 0 {
		return nil, fmt.Errorf("打开文件剪贴板失败: %w", err)
	}
	defer clipboardUser.NewProc("CloseClipboard").Call()
	handle, _, err := clipboardUser.NewProc("GetClipboardData").Call(15)
	if handle == 0 {
		return nil, err
	}
	query := clipboardShell.NewProc("DragQueryFileW")
	count, _, _ := query.Call(handle, 0xffffffff, 0, 0)
	var paths []string
	for i := uintptr(0); i < count; i++ {
		length, _, _ := query.Call(handle, i, 0, 0)
		buf := make([]uint16, length+1)
		n, _, err := query.Call(handle, i, uintptr(unsafe.Pointer(&buf[0])), length+1)
		if n == 0 {
			return nil, err
		}
		paths = append(paths, windows.UTF16ToString(buf))
	}
	return paths, nil
}
