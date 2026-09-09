package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"io"
	"strings"
	"testing"
)

func TestAppearanceRestoresIndependentOfSystemTheme(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setAppearance(app, false)
	restoreTheme(app)
	light := app.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantDark)
	for _, dark := range []bool{true, false, true} {
		setAppearance(app, dark)
		chosen := app.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantLight)
		app.Settings().SetTheme(theme.DefaultTheme())
		restoreTheme(app)
		if got := app.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantDark); got != chosen {
			t.Fatal("saved appearance lost")
		}
		if (chosen != light) != dark {
			t.Fatal("appearance follows system instead of selection")
		}
	}
	for _, dark := range []bool{true, false} {
		th := NewTheme(dark)
		for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			if th.Color("terminalForeground", variant) == th.Color("terminalBackground", variant) {
				t.Fatal("terminal text invisible")
			}
		}
	}
}

func TestTerminalSurvivesAppearanceChange(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setAppearance(app, false)
	view := terminal.NewView(io.Discard, strings.NewReader(""))
	// This is a static renderer test; the test driver executes queued UI calls inline.
	view.Close()
	view.Core.Write([]byte("中文 e\u0301 \x1b[7mreverse\x1b[0m"))
	window := app.NewWindow("terminal")
	defer window.Close()
	window.SetContent(view)
	window.Resize(fyne.NewSize(640, 400))
	window.Show()
	before := strings.Join(view.Core.Lines(), "\n")
	light := window.Canvas().Capture().At(630, 390)
	setAppearance(app, true)
	view.Refresh()
	dark := window.Canvas().Capture().At(630, 390)
	if light != dark {
		t.Fatal("prototype terminal must retain its dark background across workspace themes")
	}
	if strings.Join(view.Core.Lines(), "\n") != before {
		t.Fatal("theme switch changed terminal content")
	}
}
