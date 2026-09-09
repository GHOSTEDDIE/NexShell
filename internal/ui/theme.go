package ui

import (
	"embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

//go:embed assets/*.otf
var fonts embed.FS

func fontAsset(name string) fyne.Resource {
	b, err := fonts.ReadFile("assets/" + name)
	if err != nil {
		panic(err)
	}
	return fyne.NewStaticResource(name, b)
}

var fontResource = fontAsset("NotoSansMonoCJKsc-Regular.otf")
var uiFont = fontAsset("NotoSansCJKsc-Regular.otf")
var uiBold = fontAsset("NotoSansCJKsc-Medium.otf")

const themePreference = "appearance.theme"
const sizeBody fyne.ThemeSizeName = "body"
const sizeMeta fyne.ThemeSizeName = "meta"
const sizeTitle fyne.ThemeSizeName = "title"
const sizeControl fyne.ThemeSizeName = "control"
const colorPanel fyne.ThemeColorName = "panel"
const colorSoft fyne.ThemeColorName = "soft"
const colorMuted fyne.ThemeColorName = "muted"
const colorTrack fyne.ThemeColorName = "track"
const colorMeter fyne.ThemeColorName = "meter"
const colorMessage fyne.ThemeColorName = "message"
const colorSuccessBG fyne.ThemeColorName = "successBackground"

type desktopTheme struct {
	fyne.Theme
	mode  string
	scale float32
}

func (t desktopTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Symbol {
		return t.Theme.Font(s)
	}
	if s.Monospace {
		return fontResource
	}
	if s.Bold {
		return uiBold
	}
	return uiFont
}
func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

var lightPalette = map[fyne.ThemeColorName]color.NRGBA{
	theme.ColorNameBackground: hex(0xffffff), theme.ColorNameForeground: hex(0x33445c),
	theme.ColorNameButton: hex(0xffffff), theme.ColorNameInputBackground: hex(0xffffff),
	theme.ColorNameHeaderBackground: hex(0xf5f7fa), theme.ColorNameSeparator: hex(0xe1e7ef),
	theme.ColorNameInputBorder: hex(0xd9e1ec), theme.ColorNamePrimary: hex(0x376ed1),
	theme.ColorNameSelection: hex(0xe5edfc), theme.ColorNameHover: hex(0xeaf0f8),
	theme.ColorNameDisabled: hex(0x748399), theme.ColorNamePlaceHolder: hex(0x748399),
	theme.ColorNameSuccess: hex(0x368361), theme.ColorNameScrollBar: hex(0xd6dfea),
	colorPanel: hex(0xf5f7fa), colorSoft: hex(0xfafbfd), colorMuted: hex(0x748399),
	colorTrack: hex(0xe1e8f2), colorMeter: hex(0x789cda), colorMessage: hex(0xeaf0fc), colorSuccessBG: hex(0xedf7f0),
	"terminalSelection": hex(0x35557b), "actionForeground": hex(0xffffff), "terminalBackground": hex(0x171e29), "terminalForeground": hex(0xd5dce7),
}
var darkPalette = map[fyne.ThemeColorName]color.NRGBA{
	theme.ColorNameBackground: hex(0x1c2532), theme.ColorNameForeground: hex(0xdce4f0),
	theme.ColorNameButton: hex(0x1c2532), theme.ColorNameInputBackground: hex(0x1c2532),
	theme.ColorNameHeaderBackground: hex(0x202a38), theme.ColorNameSeparator: hex(0x354256),
	theme.ColorNameInputBorder: hex(0x354256), theme.ColorNamePrimary: hex(0x91b6fa),
	theme.ColorNameSelection: hex(0x2b3e5b), theme.ColorNameHover: hex(0x2e3d51),
	theme.ColorNameDisabled: hex(0x92a2b9), theme.ColorNamePlaceHolder: hex(0x92a2b9),
	theme.ColorNameSuccess: hex(0x91caae), theme.ColorNameScrollBar: hex(0x37465b),
	colorPanel: hex(0x202a38), colorSoft: hex(0x222d3c), colorMuted: hex(0x92a2b9),
	colorTrack: hex(0x37465b), colorMeter: hex(0x82a7e9), colorMessage: hex(0x2b3e5b), colorSuccessBG: hex(0x263d35),
	"terminalSelection": hex(0x35557b), "actionForeground": hex(0xffffff), "terminalBackground": hex(0x171e29), "terminalForeground": hex(0xd5dce7),
}

func (t desktopTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if t.mode == "dark" {
		v = theme.VariantDark
	} else if t.mode == "light" {
		v = theme.VariantLight
	}
	p := lightPalette
	if v == theme.VariantDark {
		p = darkPalette
	}
	if c, ok := p[n]; ok {
		return c
	}
	return t.Theme.Color(n, v)
}
func (t desktopTheme) Size(n fyne.ThemeSizeName) float32 {
	var s float32
	switch n {
	case theme.SizeNameText, sizeControl:
		s = 13
	case sizeBody, theme.SizeNameSubHeadingText:
		s = 14
	case sizeMeta, theme.SizeNameCaptionText:
		s = 12
	case sizeTitle, theme.SizeNameHeadingText:
		s = 18
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameModalBlurRadius:
		return 0
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 5
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius:
		return 8
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.Theme.Size(n)
	}
	return s * t.scale
}
func NewTheme(dark bool) fyne.Theme {
	mode := "light"
	if dark {
		mode = "dark"
	}
	return desktopTheme{theme.DefaultTheme(), mode, 1}
}
func restoreTheme(a fyne.App) {
	mode := a.Preferences().StringWithFallback(themePreference, "system")
	scale := float32(a.Preferences().FloatWithFallback("appearance.textScale", 1))
	a.Settings().SetTheme(desktopTheme{theme.DefaultTheme(), mode, scale})
}
func setAppearanceMode(a fyne.App, mode string) {
	a.Preferences().SetString(themePreference, mode)
	restoreTheme(a)
}
func setAppearance(a fyne.App, dark bool) {
	mode := "light"
	if dark {
		mode = "dark"
	}
	setAppearanceMode(a, mode)
}
