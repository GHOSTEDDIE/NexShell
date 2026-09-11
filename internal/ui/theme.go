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
	theme.ColorNameBackground: hex(0xffffff), theme.ColorNameForeground: hex(0x1d1d1f),
	theme.ColorNameButton: hex(0xffffff), theme.ColorNameInputBackground: hex(0xffffff),
	theme.ColorNameHeaderBackground: hex(0xf5f5f7), theme.ColorNameSeparator: hex(0xdededf),
	theme.ColorNameInputBorder: hex(0xd1d1d6), theme.ColorNamePrimary: hex(0x0066cc),
	theme.ColorNameSelection: hex(0xe3efff), theme.ColorNameHover: hex(0xebebf0),
	theme.ColorNameFocus:    hex(0xd9eaff),
	theme.ColorNameDisabled: hex(0x86868b), theme.ColorNamePlaceHolder: hex(0x76767b),
	theme.ColorNameSuccess: hex(0x1f7a35), theme.ColorNameScrollBar: hex(0xc7c7cc),
	theme.ColorNameMenuBackground: hex(0xf7f7f8), theme.ColorNameOverlayBackground: hex(0xf7f7f8),
	colorPanel: hex(0xf5f5f7), colorSoft: hex(0xfafafa), colorMuted: hex(0x6e6e73),
	colorTrack: hex(0xe5e5ea), colorMeter: hex(0x64a5ed), colorMessage: hex(0xeaf2ff), colorSuccessBG: hex(0xeaf6ed),
	"terminalSelection": hex(0x35557b), "actionForeground": hex(0xffffff), "actionBackground": hex(0x0066cc), "terminalBackground": hex(0x171e29), "terminalForeground": hex(0xd5dce7),
}
var darkPalette = map[fyne.ThemeColorName]color.NRGBA{
	theme.ColorNameBackground: hex(0x1c1c1e), theme.ColorNameForeground: hex(0xf5f5f7),
	theme.ColorNameButton: hex(0x2c2c2e), theme.ColorNameInputBackground: hex(0x232325),
	theme.ColorNameHeaderBackground: hex(0x252527), theme.ColorNameSeparator: hex(0x3a3a3c),
	theme.ColorNameInputBorder: hex(0x48484a), theme.ColorNamePrimary: hex(0x70b7ff),
	theme.ColorNameSelection: hex(0x253e59), theme.ColorNameHover: hex(0x353537),
	theme.ColorNameFocus:    hex(0x253e59),
	theme.ColorNameDisabled: hex(0x98989d), theme.ColorNamePlaceHolder: hex(0x98989d),
	theme.ColorNameSuccess: hex(0x6dd58c), theme.ColorNameScrollBar: hex(0x545456),
	theme.ColorNameMenuBackground: hex(0x2c2c2e), theme.ColorNameOverlayBackground: hex(0x2c2c2e),
	colorPanel: hex(0x252527), colorSoft: hex(0x222224), colorMuted: hex(0xaeaeb2),
	colorTrack: hex(0x3a3a3c), colorMeter: hex(0x70b7ff), colorMessage: hex(0x26384c), colorSuccessBG: hex(0x243d2b),
	"terminalSelection": hex(0x35557b), "actionForeground": hex(0xffffff), "actionBackground": hex(0x0066cc), "terminalBackground": hex(0x171e29), "terminalForeground": hex(0xd5dce7),
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
		return 7
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius:
		return 10
	case theme.SizeNameMenuRadius:
		return 5
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
