package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
)

type fileColumns struct{}

func (fileColumns) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(420, 36) }
func (fileColumns) Layout(o []fyne.CanvasObject, s fyne.Size) {
	x := []float32{18, s.Width * .58, s.Width * .76}
	width := []float32{s.Width*.58 - 30, s.Width*.18 - 12, s.Width*.24 - 18}
	for i, obj := range o {
		obj.Move(fyne.NewPos(x[i], 0))
		obj.Resize(fyne.NewSize(max(0, width[i]), s.Height))
	}
}

type fileRow struct {
	widget.BaseWidget
	name, size, modified *textView
	icon                 *widget.Icon
	content              *fyne.Container
	open, selectFile     func()
	menu                 func(fyne.Position)
}

func newFileRow() *fileRow {
	r := &fileRow{name: bodyText(""), size: metaText(""), modified: metaText(""), icon: widget.NewIcon(designIcon("file"))}
	r.content = container.New(fileColumns{}, container.NewBorder(nil, nil, inset(r.icon, 0, 10, 0, 0), nil, r.name), r.size, r.modified)
	r.ExtendBaseWidget(r)
	return r
}
func (r *fileRow) setEntry(f remote.FileEntry) {
	r.name.SetText(f.Name)
	r.size.SetText(diskSize(float64(f.Size) / 1024))
	r.icon.SetResource(designIcon("file"))
	if f.IsDir {
		r.size.SetText("—")
		r.icon.SetResource(designIcon("folder"))
	}
	r.modified.SetText(f.Modified.Format("01-02 15:04"))
}
func (r *fileRow) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.content) }
func (r *fileRow) Tapped(*fyne.PointEvent) {
	if r.selectFile != nil {
		r.selectFile()
	}
}
func (r *fileRow) DoubleTapped(*fyne.PointEvent) {
	if r.open != nil {
		r.open()
	}
}
func (r *fileRow) TappedSecondary(e *fyne.PointEvent) {
	if r.menu != nil {
		r.menu(e.AbsolutePosition)
	}
}
