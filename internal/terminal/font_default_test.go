package terminal

import (
	"fyne.io/fyne/v2/test"
	"io"
	"strings"
	"testing"
)

func TestDefaultTerminalFontIsTwelve(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	v := NewView(io.Discard, strings.NewReader(""))
	defer v.Close()
	if v.fontSize != 12 {
		t.Fatalf("default terminal font = %.0f, want 12", v.fontSize)
	}
}
