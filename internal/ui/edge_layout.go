package ui

import "fyne.io/fyne/v2"

// edge removes implicit theme gaps only at structural boundaries. Interior
// control spacing remains explicit, so tab/header baselines stay aligned.
type edgeLayout struct{ top, bottom, left, right fyne.CanvasObject }

func edge(top, bottom, left, right fyne.CanvasObject, center ...fyne.CanvasObject) *fyne.Container {
	objects := append([]fyne.CanvasObject(nil), center...)
	for _, o := range []fyne.CanvasObject{top, bottom, left, right} {
		if o != nil {
			objects = append(objects, o)
		}
	}
	return fyne.NewContainerWithLayout(edgeLayout{top, bottom, left, right}, objects...)
}
func (l edgeLayout) Layout(objects []fyne.CanvasObject, s fyne.Size) {
	t, b, left, right := float32(0), float32(0), float32(0), float32(0)
	if l.top != nil && l.top.Visible() {
		t = l.top.MinSize().Height
		l.top.Move(fyne.NewPos(0, 0))
		l.top.Resize(fyne.NewSize(s.Width, t))
	}
	if l.bottom != nil && l.bottom.Visible() {
		b = l.bottom.MinSize().Height
		l.bottom.Move(fyne.NewPos(0, max(0, s.Height-b)))
		l.bottom.Resize(fyne.NewSize(s.Width, b))
	}
	h := max(0, s.Height-t-b)
	if l.left != nil && l.left.Visible() {
		left = l.left.MinSize().Width
		l.left.Move(fyne.NewPos(0, t))
		l.left.Resize(fyne.NewSize(left, h))
	}
	if l.right != nil && l.right.Visible() {
		right = l.right.MinSize().Width
		l.right.Move(fyne.NewPos(max(0, s.Width-right), t))
		l.right.Resize(fyne.NewSize(right, h))
	}
	for _, o := range objects {
		if o != nil && o.Visible() && o != l.top && o != l.bottom && o != l.left && o != l.right {
			o.Move(fyne.NewPos(left, t))
			o.Resize(fyne.NewSize(max(0, s.Width-left-right), h))
		}
	}
}
func (l edgeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objects {
		if o != nil && o.Visible() && o != l.top && o != l.bottom && o != l.left && o != l.right {
			s = s.Max(o.MinSize())
		}
	}
	for _, o := range []fyne.CanvasObject{l.left, l.right} {
		if o != nil && o.Visible() {
			s.Width += o.MinSize().Width
			s.Height = max(s.Height, o.MinSize().Height)
		}
	}
	for _, o := range []fyne.CanvasObject{l.top, l.bottom} {
		if o != nil && o.Visible() {
			s.Height += o.MinSize().Height
			s.Width = max(s.Width, o.MinSize().Width)
		}
	}
	return s
}
