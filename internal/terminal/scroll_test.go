package terminal

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

type scrollTestTheme struct{ fyne.Theme }

func (t scrollTestTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case "terminalBackground":
		return color.Black
	case "terminalForeground":
		return color.White
	case "terminalSelection":
		return color.Gray{Y: 128}
	default:
		return t.Theme.Color(name, variant)
	}
}

func TestWheelScrollUsesReadableLineSteps(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(scrollTestTheme{theme.DarkTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	for i := 0; i < 120; i++ {
		_, _ = v.Core.Write([]byte(fmt.Sprintf("line %d\r\n", i)))
	}
	v.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, 10)})
	if v.offset < 1 || v.offset > 3 {
		t.Fatalf("one wheel event moved %d lines; want 1-3", v.offset)
	}
}

func TestScrollbarDragNavigatesHistory(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(scrollTestTheme{theme.DarkTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	win := app.NewWindow("scrollback")
	defer win.Close()
	win.SetContent(v)
	win.Resize(fyne.NewSize(600, 400))
	win.Show()
	for i := 0; i < 120; i++ {
		_, _ = v.Core.Write([]byte(fmt.Sprintf("line %d\r\n", i)))
	}
	v.Refresh()
	r := v.CreateRenderer().(*renderer)
	r.Layout(v.Size())
	if !r.scrollTrack.Visible() || !r.scrollThumb.Visible() || r.scrollThumb.Size().Height >= v.Size().Height {
		t.Fatal("scrollbar is not visible for history")
	}
	x := v.Size().Width - scrollbarWidth/2
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, 2)}, Button: desktop.MouseButtonPrimary})
	if !v.scrollbarDragging || v.offset < 100 {
		t.Fatalf("track click did not reach old history: offset=%d", v.offset)
	}
	v.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, v.Size().Height-2)}})
	if v.offset > 2 {
		t.Fatalf("thumb drag did not return to latest output: offset=%d", v.offset)
	}
	v.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if v.scrollbarDragging {
		t.Fatal("scrollbar drag remained active")
	}
	_, _ = v.Core.Write([]byte("\x1b[?1049h"))
	r.Refresh()
	if r.scrollTrack.Visible() || r.scrollThumb.Visible() {
		t.Fatal("scrollbar is visible in alternate screen")
	}
}
