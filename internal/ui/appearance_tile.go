package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type appearanceTile struct {
	widget.BaseWidget
	label, mode string
	selected    bool
	choose      func()
}

func newAppearanceTile(label, mode string, choose func()) *appearanceTile {
	t := &appearanceTile{label: label, mode: mode, choose: choose}
	t.ExtendBaseWidget(t)
	return t
}
func (t *appearanceTile) Tapped(*fyne.PointEvent) { t.choose() }
func (t *appearanceTile) CreateRenderer() fyne.WidgetRenderer {
	r := &tileRenderer{tile: t, bg: canvas.NewRectangle(theme.BackgroundColor()), image: canvas.NewImageFromResource(themePreview(t.mode)), text: canvas.NewText(t.label, theme.ForegroundColor()), radio: canvas.NewCircle(theme.BackgroundColor()), dot: canvas.NewCircle(theme.PrimaryColor())}
	r.image.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type tileRenderer struct {
	tile       *appearanceTile
	bg         *canvas.Rectangle
	image      *canvas.Image
	text       *canvas.Text
	radio, dot *canvas.Circle
}

func (r *tileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.image, r.text, r.radio, r.dot}
}
func (r *tileRenderer) Destroy()           {}
func (r *tileRenderer) MinSize() fyne.Size { return fyne.NewSize(140, 130) }
func (r *tileRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	r.image.Move(fyne.NewPos(10, 10))
	r.image.Resize(fyne.NewSize(s.Width-20, 75))
	r.text.Move(fyne.NewPos(10, 97))
	r.radio.Move(fyne.NewPos(s.Width-26, 101))
	r.radio.Resize(fyne.NewSize(13, 13))
	r.dot.Move(fyne.NewPos(s.Width-23, 104))
	r.dot.Resize(fyne.NewSize(7, 7))
}
func (r *tileRenderer) Refresh() {
	r.bg.FillColor = theme.BackgroundColor()
	r.bg.CornerRadius = 7
	r.bg.StrokeWidth = 1
	r.bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	r.text.TextSize = theme.Size(sizeControl)
	r.text.Color = theme.ForegroundColor()
	r.radio.FillColor = theme.BackgroundColor()
	r.radio.StrokeColor = theme.Color(colorMuted)
	r.radio.StrokeWidth = 1
	r.dot.FillColor = theme.PrimaryColor()
	r.dot.Hide()
	if r.tile.selected {
		r.bg.StrokeWidth = 2
		r.bg.StrokeColor = theme.PrimaryColor()
		r.radio.StrokeColor = theme.PrimaryColor()
		r.dot.Show()
	}
	r.Layout(r.tile.Size())
	r.bg.Refresh()
	r.text.Refresh()
	r.radio.Refresh()
	r.dot.Refresh()
}

func (t *appearanceTile) FocusGained()   {}
func (t *appearanceTile) FocusLost()     {}
func (t *appearanceTile) TypedRune(rune) {}
func (t *appearanceTile) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeySpace {
		t.choose()
	}
}
