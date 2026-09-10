package ui

import (
	"bytes"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestBackgroundImageDecode(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 80))
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	got, data, err := decodeBackground(bytes.NewReader(b.Bytes()))
	if err != nil || got.Bounds() != img.Bounds() || !bytes.Equal(data, b.Bytes()) {
		t.Fatalf("valid background changed: %v", err)
	}
	if _, _, err = decodeBackground(bytes.NewBufferString("invalid image")); err == nil {
		t.Fatal("invalid background accepted")
	}
	if _, _, err = decodeBackground(bytes.NewReader(make([]byte, 20*1024*1024+1))); err == nil {
		t.Fatal("oversized background accepted")
	}
}

func TestTerminalBackgroundRestored(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 30, 20))
	if err = png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.Dir, backgroundFile), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	a.Preferences().SetString("terminal.background.name", "custom.png")
	u := &App{UI: a, Store: s, status: widget.NewLabel("")}
	u.restoreTerminalBackground()
	if u.terminalBackground == nil || u.terminalBackground.Bounds() != img.Bounds() {
		t.Fatal("saved background not restored")
	}
}
