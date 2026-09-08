package ui

import (
	_ "embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/NotoSansMonoCJKsc-Regular.otf
var cjkFont []byte
var fontResource = fyne.NewStaticResource("NotoSansMonoCJKsc-Regular.otf", cjkFont)

type desktopTheme struct{ fyne.Theme }

func (t desktopTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return t.Theme.Font(style)
	}
	return fontResource
}
func NewTheme(dark bool) fyne.Theme {
	if dark {
		return desktopTheme{theme.DarkTheme()}
	}
	return desktopTheme{theme.LightTheme()}
}
