package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/theme"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
	"time"
)

func (u *App) watchInputMethod() {
	if !platforminput.Available {
		return
	}
	native, ok := u.Window.(driver.NativeWindow)
	if !ok {
		return
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var previous fyne.Focusable
		var lastPosition fyne.Position
		var lastSize fyne.Size
		var lastHeight float32
		initialized := false
		for {
			select {
			case <-u.ctx.Done():
				return
			case <-ticker.C:
			}
			fyne.DoAndWait(func() {
				if u.ctx.Err() != nil {
					return
				}
				focused := u.Window.Canvas().Focused()
				focusChanged := focused != previous
				changed := previous != nil && focusChanged
				position := fyne.Position{}
				height := theme.Size(theme.SizeNameText) + 6
				if cursor, ok := focused.(interface{ CursorPosition() fyne.Position }); ok {
					position = cursor.CursorPosition()
				}
				if object, ok := focused.(fyne.CanvasObject); ok {
					position.X = max(0, min(position.X, object.Size().Width-1))
					position.Y = max(0, min(position.Y, object.Size().Height-height))
					position = position.Add(u.UI.Driver().AbsolutePositionForObject(object))
				}
				size := u.Window.Canvas().Size()
				if initialized && !focusChanged && position == lastPosition && size == lastSize && height == lastHeight {
					return
				}
				native.RunNative(func(value any) {
					if ctx, ok := value.(driver.MacWindowContext); ok && ctx.NSWindow != 0 {
						previous = focused
						lastPosition = position
						lastSize = size
						lastHeight = height
						initialized = true
						platforminput.SetAnchor(ctx.NSWindow, position.X, position.Y, 1, height, size.Width, size.Height, changed)
					}
				})
			})
		}
	}()
}
