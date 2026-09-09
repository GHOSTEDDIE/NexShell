package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
	"unicode/utf8"
)

func themedColor(n fyne.ThemeColorName) color.Color { return theme.Color(n) }

type textView struct {
	widget.BaseWidget
	Text      string
	role      fyne.ThemeSizeName
	tone      fyne.ThemeColorName
	bold      bool
	Alignment fyne.TextAlign
}

func textUI(s string, role fyne.ThemeSizeName, tone fyne.ThemeColorName, bold bool) *textView {
	v := &textView{Text: s, role: role, tone: tone, bold: bold}
	v.ExtendBaseWidget(v)
	return v
}
func bodyText(s string) *textView    { return textUI(s, sizeBody, theme.ColorNameForeground, false) }
func metaText(s string) *textView    { return textUI(s, sizeMeta, colorMuted, false) }
func headingText(s string) *textView { return textUI(s, sizeBody, theme.ColorNameForeground, true) }
func (v *textView) SetText(s string) {
	if v.Text == s {
		return
	}
	v.Text = s
	v.Refresh()
}
func (v *textView) CreateRenderer() fyne.WidgetRenderer {
	t := canvas.NewText(v.Text, themedColor(v.tone))
	r := &textRenderer{v: v, t: t}
	r.Refresh()
	return r
}

type textRenderer struct {
	v *textView
	t *canvas.Text
}

func (r *textRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.t} }
func (r *textRenderer) Destroy()                     {}
func (r *textRenderer) MinSize() fyne.Size {
	s := fyne.MeasureText(r.v.Text, theme.Size(r.v.role), fyne.TextStyle{Bold: r.v.bold})
	return fyne.NewSize(min(s.Width, 180), theme.Size(r.v.role)*1.5)
}
func (r *textRenderer) Layout(s fyne.Size) {
	label := r.v.Text
	style := fyne.TextStyle{Bold: r.v.bold}
	if s.Width > 0 && fyne.MeasureText(label, r.t.TextSize, style).Width > s.Width {
		for len(label) > 0 && fyne.MeasureText(label+"…", r.t.TextSize, style).Width > s.Width {
			_, n := utf8.DecodeLastRuneInString(label)
			label = label[:len(label)-n]
		}
		label += "…"
	}
	r.t.Text = label
	r.t.Alignment = r.v.Alignment
	r.t.Move(fyne.NewPos(0, (s.Height-r.t.MinSize().Height)/2))
	r.t.Resize(fyne.NewSize(s.Width, r.t.MinSize().Height))
	r.t.Refresh()
}
func (r *textRenderer) Refresh() {
	r.t.Color = themedColor(r.v.tone)
	r.t.TextSize = theme.Size(r.v.role)
	r.t.TextStyle = fyne.TextStyle{Bold: r.v.bold}
	r.t.Text = r.v.Text
	r.Layout(r.v.Size())
}

type insetLayout struct{ t, r, b, l float32 }

func (l insetLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	s := o[0].MinSize()
	return fyne.NewSize(s.Width+l.l+l.r, s.Height+l.t+l.b)
}
func (l insetLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	o[0].Move(fyne.NewPos(l.l, l.t))
	o[0].Resize(fyne.NewSize(max(0, s.Width-l.l-l.r), max(0, s.Height-l.t-l.b)))
}
func inset(o fyne.CanvasObject, t, r, b, l float32) *fyne.Container {
	return container.New(insetLayout{t, r, b, l}, o)
}
func padded(o fyne.CanvasObject, n float32) *fyne.Container { return inset(o, n, n, n, n) }

type fixedLayout struct{ w, h float32 }

func (l fixedLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	s := o[0].MinSize()
	if l.w > 0 {
		s.Width = l.w
	}
	if l.h > 0 {
		s.Height = l.h
	}
	return s
}
func (l fixedLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(s)
}
func sized(o fyne.CanvasObject, w, h float32) *fyne.Container {
	return container.New(fixedLayout{w, h}, o)
}
func gap(h float32) fyne.CanvasObject { return sized(layout.NewSpacer(), 1, h) }

type surface struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	tone    fyne.ThemeColorName
	border  bool
	radius  float32
}

func panel(o fyne.CanvasObject, c fyne.ThemeColorName, border bool, radius float32) *surface {
	p := &surface{Content: o, tone: c, border: border, radius: radius}
	p.ExtendBaseWidget(p)
	return p
}
func (p *surface) CreateRenderer() fyne.WidgetRenderer {
	r := &surfaceRenderer{p: p, bg: canvas.NewRectangle(themedColor(p.tone))}
	r.Refresh()
	return r
}

type surfaceRenderer struct {
	p  *surface
	bg *canvas.Rectangle
}

func (r *surfaceRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.p.Content}
}
func (r *surfaceRenderer) Destroy()           {}
func (r *surfaceRenderer) MinSize() fyne.Size { return r.p.Content.MinSize() }
func (r *surfaceRenderer) Layout(s fyne.Size) { r.bg.Resize(s); r.p.Content.Resize(s) }
func (r *surfaceRenderer) Refresh() {
	r.bg.FillColor = themedColor(r.p.tone)
	r.bg.CornerRadius = r.p.radius
	if r.p.border {
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = themedColor(theme.ColorNameSeparator)
	}
	r.bg.Refresh()
	r.p.Content.Refresh()
}

type actionButton struct {
	outlined bool
	widget.DisableableWidget
	Text                                           string
	Icon                                           fyne.Resource
	OnTapped                                       func()
	selected, hover, focus, primary, tab, document bool
	minWidth                                       float32
}

func action(s string, icon fyne.Resource, fn func()) *actionButton {
	b := &actionButton{Text: s, Icon: icon, OnTapped: fn}
	b.ExtendBaseWidget(b)
	return b
}
func (b *actionButton) SetText(s string) { b.Text = s; b.Refresh() }
func (b *actionButton) Tapped(*fyne.PointEvent) {
	if !b.Disabled() && b.OnTapped != nil {
		b.OnTapped()
	}
}
func (b *actionButton) MouseIn(*desktop.MouseEvent)    { b.hover = true; b.Refresh() }
func (b *actionButton) MouseMoved(*desktop.MouseEvent) {}
func (b *actionButton) MouseOut()                      { b.hover = false; b.Refresh() }
func (b *actionButton) FocusGained()                   { b.focus = true; b.Refresh() }
func (b *actionButton) FocusLost()                     { b.focus = false; b.Refresh() }
func (b *actionButton) TypedRune(rune)                 {}
func (b *actionButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeySpace {
		b.Tapped(nil)
	}
}
func (b *actionButton) CreateRenderer() fyne.WidgetRenderer {
	r := &actionRenderer{b: b, bg: canvas.NewRectangle(color.Transparent), text: canvas.NewText(b.Text, themedColor(theme.ColorNameForeground)), icon: canvas.NewImageFromResource(b.Icon), line: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type actionRenderer struct {
	b        *actionButton
	bg, line *canvas.Rectangle
	text     *canvas.Text
	icon     *canvas.Image
}

func (r *actionRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon, r.text, r.line}
}
func (r *actionRenderer) Destroy() {}
func (r *actionRenderer) MinSize() fyne.Size {
	if r.b.Text == "" && !r.b.tab {
		return fyne.NewSize(30, 30)
	}
	w := fyne.MeasureText(r.b.Text, theme.Size(sizeControl), fyne.TextStyle{Bold: r.b.selected}).Width + 20
	if r.b.Icon != nil {
		w += 18
		if r.b.Text != "" {
			w += 7
		}
	}
	h := float32(30)
	if r.b.tab {
		h = 38
	}
	return fyne.NewSize(max(w, r.b.minWidth), h)
}
func (r *actionRenderer) Layout(s fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(s)
	if r.b.Text == "" && !r.b.tab {
		side := min(s.Width, s.Height)
		r.bg.Resize(fyne.NewSize(side, side))
		r.bg.Move(fyne.NewPos((s.Width-side)/2, (s.Height-side)/2))
	}
	w := fyne.MeasureText(r.b.Text, theme.Size(sizeControl), r.text.TextStyle).Width
	if r.b.Icon != nil {
		w += 18
		if r.b.Text != "" {
			w += 7
		}
	}
	x := (s.Width - w) / 2
	if r.b.Icon != nil {
		r.icon.Move(fyne.NewPos(x, (s.Height-18)/2))
		r.icon.Resize(fyne.NewSize(18, 18))
		x += 25
	}
	r.text.Move(fyne.NewPos(x, (s.Height-r.text.MinSize().Height)/2))
	r.text.Resize(r.text.MinSize())
	r.line.Move(fyne.NewPos(10, s.Height-2))
	r.line.Resize(fyne.NewSize(max(0, s.Width-20), 2))
}
func (r *actionRenderer) Refresh() {
	b := r.b
	r.text.Text = b.Text
	r.text.TextSize = theme.Size(sizeControl)
	r.text.TextStyle = fyne.TextStyle{Bold: b.selected}
	r.text.Color = themedColor(theme.ColorNameForeground)
	r.bg.FillColor = color.Transparent
	r.bg.StrokeWidth = 0
	r.bg.CornerRadius = 5
	r.line.FillColor = color.Transparent
	if b.outlined {
		r.bg.FillColor = themedColor(theme.ColorNameBackground)
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = themedColor(theme.ColorNameSeparator)
	}
	if b.hover {
		r.bg.FillColor = themedColor(theme.ColorNameHover)
	}
	if b.selected {
		r.text.Color = themedColor(theme.ColorNamePrimary)
		if b.document {
			r.text.Color = themedColor(theme.ColorNameForeground)
		}
		if b.tab {
			if b.document {
				r.bg.FillColor = themedColor(theme.ColorNameBackground)
			} else {
				r.line.FillColor = themedColor(theme.ColorNamePrimary)
			}
		} else {
			r.bg.FillColor = themedColor(theme.ColorNameSelection)
		}
	}
	if b.primary {
		r.bg.FillColor = hex(0x376ed1)
		r.text.Color = color.White
	}
	if b.focus {
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = themedColor(theme.ColorNamePrimary)
	}
	if b.Disabled() {
		r.text.Color = themedColor(colorMuted)
	}
	if b.Icon == nil {
		r.icon.Hide()
	} else {
		r.icon.Resource = b.Icon
		if res, ok := b.Icon.(*lineIcon); ok && b.primary {
			r.icon.Resource = &lineIcon{source: res.source, tone: "actionForeground"}
		}
		r.icon.Show()
		r.icon.Refresh()
	}
	r.Layout(b.Size())
	r.bg.Refresh()
	r.line.Refresh()
	r.text.Refresh()
}

func outlineAction(s string, icon fyne.Resource, fn func()) *actionButton {
	b := action(s, icon, fn)
	b.outlined = true
	return b
}
