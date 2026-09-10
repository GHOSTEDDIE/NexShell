package terminal

import (
	"github.com/charmbracelet/x/ansi"
	"image/color"
)

// Standard ANSI colors are tuned for the terminal's dark background. Extended
// and true-color values remain under the remote application's control.
var readablePalette = [16]color.NRGBA{
	{160, 174, 194, 255}, {255, 145, 153, 255}, {131, 211, 151, 255}, {238, 205, 120, 255},
	{130, 170, 255, 255}, {204, 158, 245, 255}, {112, 211, 225, 255}, {218, 225, 235, 255},
	{181, 193, 211, 255}, {255, 161, 169, 255}, {166, 232, 181, 255}, {255, 226, 155, 255},
	{169, 197, 255, 255}, {226, 190, 255, 255}, {153, 231, 240, 255}, {248, 250, 255, 255},
}

func displayColor(c color.Color, foreground bool) color.Color {
	index := -1
	switch v := c.(type) {
	case ansi.BasicColor:
		index = int(v)
	case ansi.IndexedColor:
		index = int(v)
	}
	if index >= 0 && index < len(readablePalette) {
		if index == 0 && !foreground {
			return color.NRGBA{22, 31, 42, 255}
		}
		return readablePalette[index]
	}
	return c
}
