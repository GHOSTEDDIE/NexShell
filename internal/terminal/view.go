package terminal

import (
	"image"
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

const DefaultFontSize float32 = 12

type terminalResize struct {
	rows, cols int
	apply      func(int, int)
}

type View struct {
	resizeRequests  chan terminalResize
	OnFocus         func()
	ShowContextMenu func(*fyne.Menu, fyne.Position)
	widget.BaseWidget
	Core                         *Core
	OnResize                     func(int, int)
	OnError                      func(error)
	backgroundImage              image.Image
	backgroundOpacity            float64
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
	v := &View{Core: NewCore(80, 24), fontSize: DefaultFontSize, cols: 80, rows: 24, keys: make(chan func(), 256), resizeRequests: make(chan terminalResize, 1), done: make(chan struct{}), selectionStart: -1, selectionEnd: -1}
	if len(errorHandlers) > 0 {
		v.OnError = errorHandlers[0]
	}
	v.ExtendBaseWidget(v)
	v.dirty.Store(true)
	if closer, ok := output.(io.Closer); ok {
		v.source = closer
	}
	v.measure()
	go v.runResizes()
	go func() {
		_, err := io.Copy(input, v.Core)
		v.Core.CloseInput()
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
				_, writeErr := v.Core.Write(buf[:n])
				v.dirty.Store(true)
				if writeErr != nil {
					v.report(writeErr)
					return
				}
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
	sample := canvas.NewText("M", color.White)
	sample.TextSize = v.fontSize
	sample.FontSource = theme.DefaultTextMonospaceFont()
	s := sample.MinSize()
	v.cellSize = fyne.NewSize(float32(math.Ceil(float64(max(s.Width, v.fontSize*.6)))), float32(math.Ceil(float64(max(s.Height, v.fontSize*1.8)))))
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
	cols := max(2, int(size.Width/v.cellSize.Width))
	rows := max(2, int(size.Height/v.cellSize.Height))
	if cols != v.cols || rows != v.rows {
		v.clearSelection()
		v.cols = cols
		v.rows = rows
		v.Core.Resize(cols, rows)
		if v.OnResize != nil {
			v.queueResize(terminalResize{rows: rows, cols: cols, apply: v.OnResize})
		}
	}
	v.BaseWidget.Resize(size)
}
func (v *View) FocusGained() {
	v.focused = true
	if v.OnFocus != nil {
		v.OnFocus()
	}
	v.Refresh()
}
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
		if v.ShowContextMenu != nil {
			v.ShowContextMenu(menu, e.AbsolutePosition)
		} else {
			widget.ShowPopUpMenuAtPosition(menu, fyne.CurrentApp().Driver().CanvasForObject(v), e.AbsolutePosition)
		}
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
	wallpaper := canvas.NewImageFromImage(v.backgroundImage)
	wallpaper.FillMode = canvas.ImageFillCover
	return &renderer{v: v, background: canvas.NewRectangle(theme.Color("terminalBackground")), wallpaper: wallpaper}
}

type renderer struct {
	v          *View
	background *canvas.Rectangle
	wallpaper  *canvas.Image
	texts      []*canvas.Text
	backs      []*canvas.Rectangle
	objects    []fyne.CanvasObject
}

func (r *renderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.wallpaper.Resize(size)
	r.paint()
}
func (r *renderer) MinSize() fyne.Size { return r.v.MinSize() }
func (r *renderer) Refresh()           { r.paint(); canvas.Refresh(r.v) }
func (r *renderer) Objects() []fyne.CanvasObject {
	if len(r.objects) == 0 {
		r.paint()
	}
	return r.objects
}
func (r *renderer) Destroy() {}
func (r *renderer) paint() {
	v := r.v
	if r.wallpaper.Image != v.backgroundImage || r.wallpaper.Translucency != 1-v.backgroundOpacity {
		r.wallpaper.Image = v.backgroundImage
		r.wallpaper.Translucency = 1 - v.backgroundOpacity
		r.wallpaper.Refresh()
	}
	if v.backgroundImage == nil {
		r.wallpaper.Hide()
	} else {
		r.wallpaper.Show()
	}
	r.background.FillColor = theme.Color("terminalBackground")
	s := v.Core.Snapshot(v.offset)
	if v.selectionScreen != nil {
		s = *v.selectionScreen
	}
	n := v.cols * v.rows
	if len(r.texts) != n {
		r.texts = make([]*canvas.Text, n)
		r.backs = make([]*canvas.Rectangle, n)
		r.objects = []fyne.CanvasObject{r.background, r.wallpaper}
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
	textOffset := max(float32(0), (v.cellSize.Height-fyne.MeasureText("M", v.fontSize, fyne.TextStyle{Monospace: true}).Height)/2)
	for y, row := range s.Rows {
		for x, cell := range row {
			i := y*v.cols + x
			if i >= len(r.texts) {
				continue
			}
			t, b := r.texts[i], r.backs[i]
			fg, bg := displayColor(cell.Style.Fg, true), displayColor(cell.Style.Bg, false)
			if fg == nil {
				fg = theme.Color("terminalForeground")
			}
			if bg == nil {
				bg = theme.Color("terminalBackground")
			}
			if cell.Style.Attrs&uv.AttrReverse != 0 {
				fg, bg = bg, fg
			}
			if start >= 0 && i >= start && i <= end {
				bg = theme.Color("terminalSelection")
			}
			if v.focused && x == s.CursorX && y == s.CursorY {
				bg = theme.Color("terminalSelection")
			}
			if v.backgroundImage != nil && cell.Style.Bg == nil && cell.Style.Attrs&uv.AttrReverse == 0 && !(start >= 0 && i >= start && i <= end) && !(v.focused && x == s.CursorX && y == s.CursorY) {
				bg = color.Transparent
			}
			b.FillColor = bg
			b.Move(fyne.NewPos(float32(x)*v.cellSize.Width, float32(y)*v.cellSize.Height))
			b.Resize(fyne.NewSize(v.cellSize.Width*float32(max(1, cell.Width)), v.cellSize.Height))
			t.Text = cell.Text
			t.FontSource = nil
			if asciiCell(cell.Text) {
				t.FontSource = theme.DefaultTextMonospaceFont()
			}
			t.Color = fg
			t.TextSize = v.fontSize
			t.TextStyle = fyne.TextStyle{Monospace: true, Bold: cell.Style.Attrs&uv.AttrBold != 0, Italic: cell.Style.Attrs&uv.AttrItalic != 0}
			t.Move(b.Position().Add(fyne.NewPos(0, textOffset)))
			t.Resize(b.Size())
			if cell.Width == 0 {
				t.Text = ""
			}
		}
	}
}

func asciiCell(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// SetBackground applies a shared decoded image without changing terminal cells.
func (v *View) SetBackground(img image.Image, opacity float64) {
	v.backgroundImage = img
	v.backgroundOpacity = math.Max(0, math.Min(.15, opacity))
	v.Refresh()
}

// CursorPosition anchors the native input method without copying screen cells.
func (v *View) CursorPosition() fyne.Position {
	v.Core.mu.Lock()
	defer v.Core.mu.Unlock()
	pos := v.Core.vt.CursorPosition()
	return fyne.NewPos(float32(pos.X)*v.cellSize.Width, float32(pos.Y)*v.cellSize.Height)
}

// Resize requests are ordered independently of terminal input. While a peer is
// slow, retain only the latest geometry instead of spawning unbounded writers.
func (v *View) queueResize(request terminalResize) {
	select {
	case <-v.done:
		return
	default:
	}
	select {
	case <-v.resizeRequests:
	default:
	}
	select {
	case v.resizeRequests <- request:
	case <-v.done:
	}
}
func (v *View) runResizes() {
	for {
		select {
		case <-v.done:
			return
		case request := <-v.resizeRequests:
			select {
			case <-v.done:
				return
			default:
			}
			request.apply(request.rows, request.cols)
		}
	}
}
