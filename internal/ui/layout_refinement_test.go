package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestIconOnlyActionsAreSquare(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	b := action("", designIcon("settings"), func() {})
	size := b.MinSize()
	if size.Width != size.Height {
		t.Fatalf("icon control is stretched: %.0fx%.0f, want square", size.Width, size.Height)
	}
}
func TestTabStripHasNoUnrequestedGutter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	tabs := newTabView(false, container.NewTabItem("文件", widget.NewLabel("content")))
	win := app.NewWindow("layout")
	defer win.Close()
	win.SetPadded(false)
	win.SetContent(tabs)
	win.Resize(fyne.NewSize(640, 480))
	win.Show()
	if y := tabs.body.Position().Y; y != 38 {
		t.Fatalf("content starts at y=%.0f; tab strip ends at 38", y)
	}
}
