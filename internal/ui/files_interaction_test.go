package ui

import (
	"fyne.io/fyne/v2"
	"testing"
)

func TestDropBounds(t *testing.T) {
	origin := fyne.NewPos(200, 500)
	size := fyne.NewSize(800, 300)
	for _, tc := range []struct {
		pos  fyne.Position
		want bool
	}{{fyne.NewPos(200, 500), true}, {fyne.NewPos(600, 650), true}, {fyne.NewPos(199, 600), false}, {fyne.NewPos(600, 499), false}, {fyne.NewPos(1000, 600), false}, {fyne.NewPos(600, 800), false}} {
		if containsPosition(tc.pos, origin, size) != tc.want {
			t.Fatal(tc)
		}
	}
}
