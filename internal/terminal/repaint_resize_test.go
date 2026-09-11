package terminal

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"io"
	"strings"
	"testing"
)

func TestResizePaintUsesNewGridImmediately(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(readabilityTheme{theme.DefaultTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	r := test.WidgetRenderer(v).(*renderer)
	v.Resize(fyne.NewSize(420, 380))
	if len(r.texts) != v.cols*v.rows {
		t.Fatalf("old grid still painted: %d cells, want %d", len(r.texts), v.cols*v.rows)
	}
}
