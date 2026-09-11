package terminal

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"io"
	"strings"
	"testing"
)

func TestContextMenuPresenterRespectsRemoteMouseTracking(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	w := a.NewWindow("terminal menu")
	defer w.Close()
	w.SetContent(v)
	w.Show()
	pos := fyne.NewPos(20, 30)
	calls := 0
	v.ShowContextMenu = func(menu *fyne.Menu, at fyne.Position) {
		calls++
		if at != pos || len(menu.Items) != 2 || menu.Items[0].Label != "复制" || menu.Items[1].Label != "粘贴" {
			t.Fatal("terminal menu contract changed")
		}
	}
	e := &desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos, AbsolutePosition: pos}, Button: desktop.MouseButtonSecondary}
	v.MouseDown(e)
	if calls != 1 {
		t.Fatal("custom context menu was not used")
	}
	v.Core.Write([]byte("\x1b[?1000h"))
	v.MouseDown(e)
	if calls != 1 {
		t.Fatal("context menu intercepted remote mouse tracking")
	}
	e.Modifier = fyne.KeyModifierShift
	v.MouseDown(e)
	if calls != 2 {
		t.Fatal("Shift did not restore the local context menu")
	}
}
