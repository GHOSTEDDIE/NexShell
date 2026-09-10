package terminal

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
	"testing"
)

func contrast(a, b color.Color) float64 {
	lum := func(c color.Color) float64 {
		r, g, b, _ := c.RGBA()
		conv := func(v uint32) float64 {
			x := float64(v) / 65535
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return .2126*conv(r) + .7152*conv(g) + .0722*conv(b)
	}
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + .05) / (y + .05)
}
func TestTerminalANSIReadability(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(readabilityTheme{theme.DefaultTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	v.Core.Write([]byte("\x1b[34mdirectory\x1b[31marchive"))
	r := test.WidgetRenderer(v)
	r.Refresh()
	for _, o := range r.Objects() {
		if txt, ok := o.(*canvas.Text); ok && (txt.Text == "d" || txt.Text == "a") {
			if ratio := contrast(txt.Color, color.NRGBA{22, 31, 42, 255}); ratio < 4.5 {
				t.Errorf("%s contrast %.2f < 4.5", txt.Text, ratio)
			}
		}
	}
}

type readabilityTheme struct{ fyne.Theme }

func (t readabilityTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case "terminalBackground":
		return color.NRGBA{22, 31, 42, 255}
	case "terminalForeground":
		return color.NRGBA{218, 225, 235, 255}
	case "terminalSelection":
		return color.NRGBA{60, 80, 110, 255}
	}
	return t.Theme.Color(n, v)
}

func TestTerminalBackgroundPreservesCellColors(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(readabilityTheme{theme.DefaultTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	v.Close()
	v.Core.Write([]byte("plain\x1b[41mred\x1b[0m\x1b[7mreverse"))
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	v.SetBackground(img, .12)
	r := test.WidgetRenderer(v).(*renderer)
	r.Layout(fyne.NewSize(800, 400))
	r.Refresh()
	if !r.wallpaper.Visible() || r.wallpaper.Image != img {
		t.Fatal("background not shown")
	}
	_, _, _, alpha := r.backs[0].FillColor.RGBA()
	if alpha != 0 {
		t.Fatal("default cell hides wallpaper")
	}
	for _, index := range []int{5, 8} {
		_, _, _, alpha = r.backs[index].FillColor.RGBA()
		if alpha == 0 {
			t.Fatal("explicit or reversed cell background lost")
		}
	}
	v.SetBackground(nil, .12)
	r.Refresh()
	_, _, _, alpha = r.backs[0].FillColor.RGBA()
	if alpha == 0 || r.wallpaper.Visible() {
		t.Fatal("removing background did not restore solid terminal")
	}
}

func TestPaletteReadableOverBrightWallpaper(t *testing.T) {
	for i, c := range readablePalette {
		if ratio := contrast(c, color.NRGBA{57, 65, 74, 255}); ratio < 4.5 {
			t.Errorf("ANSI %d contrast over white wallpaper = %.2f", i, ratio)
		}
	}
}
