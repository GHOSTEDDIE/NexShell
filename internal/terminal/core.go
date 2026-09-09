// Package terminal isolates the experimental VT dependency from desktop and SSH services.
package terminal

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"io"
	"strings"
	"sync"
)

type Cell struct {
	Text  string
	Width int
	Style uv.Style
}
type Screen struct {
	Rows             [][]Cell
	CursorX, CursorY int
	Alt              bool
	History          int
}
type Core struct {
	mu            sync.Mutex
	vt            *vt.Emulator
	input         *inputBuffer
	cursorVisible bool
	mouseModes    map[ansi.Mode]bool
}

func NewCore(cols, rows int) *Core {
	c := &Core{vt: vt.NewEmulator(cols, rows), cursorVisible: true, mouseModes: map[ansi.Mode]bool{}}
	c.input = newInputBuffer()
	c.vt.SetInputPipe(c.input)
	c.vt.SetScrollbackSize(10000)
	c.vt.SetCallbacks(vt.Callbacks{CursorVisibility: func(visible bool) { c.cursorVisible = visible }, EnableMode: func(m ansi.Mode) {
		if m == ansi.ModeMouseX10 || m == ansi.ModeMouseNormal || m == ansi.ModeMouseButtonEvent || m == ansi.ModeMouseAnyEvent {
			c.mouseModes[m] = true
		}
	}, DisableMode: func(m ansi.Mode) { delete(c.mouseModes, m) }})
	return c
}
func (c *Core) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, err := c.vt.Write(b)
	if err == nil {
		err = c.input.Err()
	}
	return n, err
}
func (c *Core) Read(b []byte) (int, error) { return c.vt.Read(b) }
func (c *Core) CloseInput() error          { return c.vt.InputPipe().(io.Closer).Close() }
func (c *Core) Resize(cols, rows int) {
	if cols < 2 || rows < 2 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vt.Resize(cols, rows)
}
func (c *Core) Key(key uv.Key) { c.mu.Lock(); defer c.mu.Unlock(); c.vt.SendKey(uv.KeyPressEvent(key)) }
func (c *Core) Text(text string, paste bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if paste {
		c.vt.Paste(text)
	} else {
		c.vt.SendText(text)
	}
}
func (c *Core) Mouse(mouse uv.MouseEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vt.SendMouse(mouse)
}
func (c *Core) MouseTracking() bool { c.mu.Lock(); defer c.mu.Unlock(); return len(c.mouseModes) > 0 }
func (c *Core) Snapshot(offset int) Screen {
	c.mu.Lock()
	defer c.mu.Unlock()
	history := c.vt.ScrollbackLen()
	if c.vt.IsAltScreen() {
		offset = 0
		history = 0
	}
	if offset > history {
		offset = history
	}
	if offset < 0 {
		offset = 0
	}
	s := Screen{Rows: make([][]Cell, c.vt.Height()), Alt: c.vt.IsAltScreen(), History: history}
	pos := c.vt.CursorPosition()
	s.CursorX = pos.X
	s.CursorY = pos.Y + offset
	if !c.cursorVisible {
		s.CursorY = -1
	}
	for y := range s.Rows {
		s.Rows[y] = make([]Cell, c.vt.Width())
		absolute := history - offset + y
		for x := range s.Rows[y] {
			var cell *uv.Cell
			if absolute < history {
				cell = c.vt.ScrollbackCellAt(x, absolute)
			} else {
				cell = c.vt.CellAt(x, absolute-history)
			}
			if cell != nil {
				s.Rows[y][x] = Cell{cell.Content, cell.Width, cell.Style}
			}
		}
	}
	return s
}
func (c *Core) Lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	history := c.vt.ScrollbackLen()
	if c.vt.IsAltScreen() {
		history = 0
	}
	lines := make([]string, history+c.vt.Height())
	for y := range lines {
		var b strings.Builder
		for x := 0; x < c.vt.Width(); x++ {
			var cell *uv.Cell
			if y < history {
				cell = c.vt.ScrollbackCellAt(x, y)
			} else {
				cell = c.vt.CellAt(x, y-history)
			}
			if cell == nil {
				b.WriteByte(' ')
			} else if cell.Width != 0 {
				if cell.Content == "" {
					b.WriteByte(' ')
				} else {
					b.WriteString(cell.Content)
				}
			}
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return lines
}
func (c *Core) Search(query string) []int {
	if query == "" {
		return nil
	}
	var hits []int
	for i, l := range c.Lines() {
		if strings.Contains(strings.ToLower(l), strings.ToLower(query)) {
			hits = append(hits, i)
		}
	}
	return hits
}
