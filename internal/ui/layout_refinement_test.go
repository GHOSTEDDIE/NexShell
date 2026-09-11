package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
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

func TestSelectedTabsUseBlueUnderline(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	for _, document := range []bool{false, true} {
		tabs := newTabView(document,
			container.NewTabItem("当前", widget.NewLabel("one")),
			container.NewTabItem("其它", widget.NewLabel("two")),
		)
		win := app.NewWindow("tab underline")
		win.SetContent(tabs)
		win.Resize(fyne.NewSize(640, 300))
		win.Show()
		var findAction func(fyne.CanvasObject) *actionButton
		findAction = func(obj fyne.CanvasObject) *actionButton {
			if b, ok := obj.(*actionButton); ok {
				return b
			}
			if c, ok := obj.(*fyne.Container); ok {
				for _, child := range c.Objects {
					if b := findAction(child); b != nil {
						return b
					}
				}
			}
			return nil
		}
		button := findAction(tabs.strip.Objects[0])
		if button == nil {
			t.Fatal("selected tab action missing")
		}
		renderer := test.WidgetRenderer(button).(*actionRenderer)
		if renderer.line.FillColor == nil || renderer.line.FillColor != themedColor(theme.ColorNamePrimary) {
			t.Fatalf("document=%v selected tab has no accent underline", document)
		}
		if renderer.bg.FillColor != color.Transparent {
			t.Fatalf("document=%v selected tab uses a filled background", document)
		}
		unselected := findAction(tabs.strip.Objects[1])
		if unselected == nil {
			t.Fatal("unselected tab action missing")
		}
		unselectedRenderer := test.WidgetRenderer(unselected).(*actionRenderer)
		if unselectedRenderer.line.FillColor != color.Transparent {
			t.Fatalf("document=%v unselected tab has an underline", document)
		}
		win.Close()
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
