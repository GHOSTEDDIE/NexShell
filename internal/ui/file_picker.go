package ui

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"github.com/GHOSTEDDIE/nexshell/internal/nativefiles"
)

// Call on the UI thread. OS dialogs run independently of Fyne's event loop;
// every completion returns to the UI thread, including cancellation.
func (u *App) pickFiles(ctx context.Context, request nativefiles.Request, done func([]string, error)) {
	if u.filePickerOpen {
		done(nil, fmt.Errorf("请先关闭已打开的文件选择窗口"))
		return
	}
	u.filePickerOpen = true
	if window, ok := u.Window.(driver.NativeWindow); ok {
		window.RunNative(func(c any) {
			switch c := c.(type) {
			case driver.MacWindowContext:
				request.Parent = c.NSWindow
			case driver.WindowsWindowContext:
				request.Parent = c.HWND
			case driver.X11WindowContext:
				request.Parent = int(c.WindowHandle)
			}
		})
	}
	choose := u.localFilePicker
	if choose == nil {
		choose = nativefiles.Choose
	}
	go func() {
		paths, err := choose(ctx, request)
		if errors.Is(err, nativefiles.ErrCanceled) {
			paths, err = nil, nil
		}
		fyne.Do(func() {
			u.filePickerOpen = false
			if u.closing {
				return
			}
			if ctx.Err() != nil {
				paths, err = nil, ctx.Err()
			}
			done(paths, err)
		})
	}()
}
