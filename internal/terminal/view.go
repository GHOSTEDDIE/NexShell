package terminal

import (
	"image/color"
	"io"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	uv "github.com/charmbracelet/ultraviolet"
)

type View struct {
	widget.BaseWidget
	Core                         *Core
	OnResize                     func(int, int)
	OnError                      func(error)
	fontSize                     float32
	cellSize                     fyne.Size
	cols, rows                   int
	offset                       int
	focused                      bool
	selectionStart, selectionEnd int
	selecting                    bool
	selectionScreen              *Screen
	keys                         chan func()
	done                         chan struct{}
	once                         sync.Once
	source                       io.Closer
	modifier                     uv.KeyMod
	dirty                        atomic.Bool
}

func NewView(input io.Writer, output io.Reader, errorHandlers ...func(error)) *View {
	v := &View{Core: NewCore(80, 24), fontSize: 14, cols: 80, rows: 24, keys: make(chan func(), 256), done: make(chan struct{}), selectionStart: -1, selectionEnd: -1}
	if len(errorHandlers) > 0 {
		v.OnError = errorHandlers[0]
	}
	v.ExtendBaseWidget(v)
	v.dirty.Store(true)
	if closer, ok := output.(io.Closer); ok {
		v.source = closer
	}
	v.measure()
	go func() {
		_, err := io.Copy(input, v.Core)
		if err != nil {
			v.report(err)
		}
	}()
	go func() {
		for {
			select {
			case f := <-v.keys:
				f()
			case <-v.done:
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := output.Read(buf)
			if n > 0 {
				_, _ = v.Core.Write(buf[:n])
				v.dirty.Store(true)
			}
			if err != nil {
				if err != io.EOF {
					v.report(err)
				}
				return
			}
		}
	}()
	go func() {
		t := time.NewTicker(time.Second / 30)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if !v.dirty.Swap(false) {
					continue
				}
				fyne.Do(func() {
					select {
					case <-v.done:
						return
					default:
					}
					v.Refresh()
				})
			case <-v.done:
				return
			}
		}
	}()
	return v
}
func (v *View) report(err error) {
	if v.OnError != nil {
		fyne.Do(func() { v.OnError(err) })
	}
}
func (v *View) Close() {
	v.once.Do(func() {
		close(v.done)
		v.Core.CloseInput()
		if v.source != nil {
			v.source.Close()
		}
	})
}
func (v *View) measure() {
	s := fyne.MeasureText("M", v.fontSize, fyne.TextStyle{Monospace: true})
	v.cellSize = fyne.NewSize(float32(math.Ceil(float64(s.Width))), float32(math.Ceil(float64(s.Height))))
}
func (v *View) SetFontSize(size float32) {
	if size < 8 || size > 40 {
		return
	}
	v.fontSize = size
	v.measure()
	v.Resize(v.Size())
	v.Refresh()
}
func (v *View) MinSize() fyne.Size { return fyne.NewSize(240, 150) }
func (v *View) Resize(size fyne.Size) {
	v.BaseWidget.Resize(size)
	cols := max(2, int(size.Width/v.cellSize.Width))
	rows := max(2, int(size.Height/v.cellSize.Height))
	if cols != v.cols || rows != v.rows {
		v.clearSelection()
		v.cols = cols
		v.rows = rows
		v.Core.Resize(cols, rows)
		if v.OnResize != nil {
			go v.OnResize(rows, cols)
		}
	}
}
func (v *View) FocusGained()     { v.focused = true; v.Refresh() }
func (v *View) FocusLost()       { v.focused = false; v.modifier = 0; v.Refresh() }
func (v *View) AcceptsTab() bool { return true }
func (v *View) enqueue(f func()) {
	select {
	case <-v.done:
		return
	default:
	}
	select {
	case v.keys <- f:
	default:
		v.report(io.ErrShortWrite)
	}
}
func (v *View) TypedRune(r rune) {
	v.clearSelection()
	v.offset = 0
	v.enqueue(func() { v.Core.Text(string(r), false) })
}

var special = map[fyne.KeyName]rune{fyne.KeyReturn: uv.KeyEnter, fyne.KeyEnter: uv.KeyEnter, fyne.KeyTab: uv.KeyTab, fyne.KeyBackspace: uv.KeyBackspace, fyne.KeyDelete: uv.KeyDelete, fyne.KeyEscape: uv.KeyEscape, fyne.KeyUp: uv.KeyUp, fyne.KeyDown: uv.KeyDown, fyne.KeyLeft: uv.KeyLeft, fyne.KeyRight: uv.KeyRight, fyne.KeyHome: uv.KeyHome, fyne.KeyEnd: uv.KeyEnd, fyne.KeyPageUp: uv.KeyPgUp, fyne.KeyPageDown: uv.KeyPgDown, fyne.KeyF1: uv.KeyF1, fyne.KeyF2: uv.KeyF2, fyne.KeyF3: uv.KeyF3, fyne.KeyF4: uv.KeyF4, fyne.KeyF5: uv.KeyF5, fyne.KeyF6: uv.KeyF6, fyne.KeyF7: uv.KeyF7, fyne.KeyF8: uv.KeyF8, fyne.KeyF9: uv.KeyF9, fyne.KeyF10: uv.KeyF10, fyne.KeyF11: uv.KeyF11, fyne.KeyF12: uv.KeyF12}

func (v *View) TypedKey(e *fyne.KeyEvent) {
	v.clearSelection()
	code, ok := special[e.Name]
	if !ok {
		return
	}
	mod := v.modifier
	v.offset = 0
	v.enqueue(func() { v.Core.Key(uv.Key{Code: code, Mod: mod}) })
}
func (v *View) TypedShortcut(s fyne.Shortcut) {
	switch sh := s.(type) {
	case *fyne.ShortcutCopy:
		v.Copy()
	case *fyne.ShortcutPaste:
		text := sh.Clipboard.Content()
		v.enqueue(func() { v.Core.Text(text, true) })
	case *desktop.CustomShortcut:
		if sh.Modifier&fyne.KeyModifierControl != 0 && sh.Modifier&fyne.KeyModifierShift == 0 {
			text := strings.ToLower(string(sh.KeyName))
			if len(text) == 1 {
				v.enqueue(func() { v.Core.Key(uv.Key{Code: rune(text[0]), Mod: uv.ModCtrl}) })
			}
		}
	}
}
func (v *View) KeyDown(e *fyne.KeyEvent) {
	switch e.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		v.modifier |= uv.ModShift
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		v.modifier |= uv.ModCtrl
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		v.modifier |= uv.ModAlt
	}
}
func (v *View) KeyUp(e *fyne.KeyEvent) {
	switch e.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		v.modifier &^= uv.ModShift
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		v.modifier &^= uv.ModCtrl
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		v.modifier &^= uv.ModAlt
	}
}
func (v *View) Copy() {
	s := v.Core.Snapshot(v.offset)
	if v.selectionScreen != nil {
		s = *v.selectionScreen
	}
	start, end := v.selectionStart, v.selectionEnd
	if start < 0 || end < 0 {
		return
	}
	if start > end {
		start, end = end, start
	}
	var b strings.Builder
	for y, row := range s.Rows {
		for x, cell := range row {
			i := y*v.cols + x
			if i < start || i > end || cell.Width == 0 {
				continue
			}
			if cell.Text == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(cell.Text)
			}
		}
		if y*v.cols <= end && (y+1)*v.cols > start && y*v.cols < end {
			b.WriteByte('\n')
		}
	}
	fyne.CurrentApp().Clipboard().SetContent(strings.TrimSuffix(b.String(), "\n"))
}
func (v *View) position(p fyne.Position) int {
	x := min(v.cols-1, max(0, int(p.X/v.cellSize.Width)))
	y := min(v.rows-1, max(0, int(p.Y/v.cellSize.Height)))
	return y*v.cols + x
}
func (v *View) Tapped(e *fyne.PointEvent) { fyne.CurrentApp().Driver().CanvasForObject(v).Focus(v) }
func (v *View) MouseDown(e *desktop.MouseEvent) {
	v.Tapped(&fyne.PointEvent{Position: e.Position})
	if v.Core.MouseTracking() && e.Modifier&fyne.KeyModifierShift == 0 {
		mouse := v.mouseAt(e.Position)
		if e.Button == desktop.MouseButtonSecondary {
			mouse.Button = uv.MouseRight
		}
		v.enqueue(func() { v.Core.Mouse(uv.MouseClickEvent(mouse)) })
		return
	}
	if e.Button == desktop.MouseButtonPrimary {
		screen := v.Core.Snapshot(v.offset)
		v.selectionScreen = &screen
		v.selectionStart = v.position(e.Position)
		v.selectionEnd = v.selectionStart
		v.selecting = true
	}
	if e.Button == desktop.MouseButtonSecondary {
		menu := fyne.NewMenu("", fyne.NewMenuItem("复制", v.Copy), fyne.NewMenuItem("粘贴", func() { text := fyne.CurrentApp().Clipboard().Content(); v.enqueue(func() { v.Core.Text(text, true) }) }))
		widget.ShowPopUpMenuAtPosition(menu, fyne.CurrentApp().Driver().CanvasForObject(v), e.AbsolutePosition)
	}
}
func (v *View) MouseUp(e *desktop.MouseEvent) {
	if !v.selecting && v.Core.MouseTracking() {
		mouse := v.mouseAt(e.Position)
		v.enqueue(func() { v.Core.Mouse(uv.MouseReleaseEvent(mouse)) })
	}
	v.selecting = false
}
func (v *View) Dragged(e *fyne.DragEvent) {
	if !v.selecting && v.Core.MouseTracking() {
		mouse := v.mouseAt(e.Position)
		v.enqueue(func() { v.Core.Mouse(uv.MouseMotionEvent(mouse)) })
		return
	}
	if v.selecting {
		v.selectionEnd = v.position(e.Position)
		v.Refresh()
	}
}
func (v *View) DragEnd() { v.selecting = false }
func (v *View) Scrolled(e *fyne.ScrollEvent) {
	if v.Core.MouseTracking() && v.modifier&uv.ModShift == 0 {
		mouse := v.mouseAt(e.Position)
		mouse.Button = uv.MouseWheelDown
		if e.Scrolled.DY > 0 {
			mouse.Button = uv.MouseWheelUp
		}
		v.enqueue(func() { v.Core.Mouse(uv.MouseWheelEvent(mouse)) })
		return
	}
	v.clearSelection()
	s := v.Core.Snapshot(v.offset)
	v.offset = max(0, min(s.History, v.offset+int(e.Scrolled.DY)*3))
	v.selectionStart = -1
	v.selectionEnd = -1
	v.Refresh()
}
func (v *View) Search(query string) int {
	hits := v.Core.Search(query)
	if len(hits) > 0 {
		s := v.Core.Snapshot(0)
		v.offset = max(0, s.History-hits[len(hits)-1])
		v.Refresh()
	}
	return len(hits)
}
func (v *View) clearSelection() { v.selectionScreen = nil; v.selectionStart = -1; v.selectionEnd = -1 }
func (v *View) mouseAt(p fyne.Position) uv.Mouse {
	return uv.Mouse{X: max(0, int(p.X/v.cellSize.Width)), Y: max(0, int(p.Y/v.cellSize.Height)), Button: uv.MouseLeft, Mod: v.modifier}
}
func (v *View) Send(text string) { v.enqueue(func() { v.Core.Text(text, false) }) }
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return &renderer{v: v, background: canvas.NewRectangle(color.NRGBA{R: 16, G: 22, B: 32, A: 255})}
}

type renderer struct {
	v          *View
	background *canvas.Rectangle
	texts      []*canvas.Text
	backs      []*canvas.Rectangle
	objects    []fyne.CanvasObject
}

func (r *renderer) Layout(size fyne.Size) { r.background.Resize(size); r.paint() }
func (r *renderer) MinSize() fyne.Size    { return r.v.MinSize() }
func (r *renderer) Refresh()              { r.paint(); canvas.Refresh(r.v) }
func (r *renderer) Objects() []fyne.CanvasObject {
	if len(r.objects) == 0 {
		r.paint()
	}
	return r.objects
}
func (r *renderer) Destroy() {}
func (r *renderer) paint() {
	v := r.v
	s := v.Core.Snapshot(v.offset)
	if v.selectionScreen != nil {
		s = *v.selectionScreen
	}
	n := v.cols * v.rows
	if len(r.texts) != n {
		r.texts = make([]*canvas.Text, n)
		r.backs = make([]*canvas.Rectangle, n)
		r.objects = []fyne.CanvasObject{r.background}
		for i := 0; i < n; i++ {
			r.backs[i] = canvas.NewRectangle(color.Transparent)
			r.texts[i] = canvas.NewText("", color.White)
			r.objects = append(r.objects, r.backs[i], r.texts[i])
		}
	}
	start, end := v.selectionStart, v.selectionEnd
	if start > end {
		start, end = end, start
	}
	for y, row := range s.Rows {
		for x, cell := range row {
			i := y*v.cols + x
			if i >= len(r.texts) {
				continue
			}
			t, b := r.texts[i], r.backs[i]
			fg, bg := cell.Style.Fg, cell.Style.Bg
			if fg == nil {
				fg = color.NRGBA{R: 219, G: 229, B: 242, A: 255}
			}
			if bg == nil {
				bg = color.Transparent
			}
			if cell.Style.Attrs&uv.AttrReverse != 0 {
				fg, bg = bg, fg
			}
			if start >= 0 && i >= start && i <= end {
				bg = theme.SelectionColor()
			}
			if v.focused && x == s.CursorX && y == s.CursorY {
				bg = color.NRGBA{R: 56, G: 102, B: 151, A: 255}
			}
			b.FillColor = bg
			b.Move(fyne.NewPos(float32(x)*v.cellSize.Width, float32(y)*v.cellSize.Height))
			b.Resize(fyne.NewSize(v.cellSize.Width*float32(max(1, cell.Width)), v.cellSize.Height))
			t.Text = cell.Text
			t.Color = fg
			t.TextSize = v.fontSize
			t.TextStyle = fyne.TextStyle{Monospace: true, Bold: cell.Style.Attrs&uv.AttrBold != 0, Italic: cell.Style.Attrs&uv.AttrItalic != 0}
			t.Move(b.Position())
			t.Resize(b.Size())
			if cell.Width == 0 {
				t.Text = ""
			}
		}
	}
}
