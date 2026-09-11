package vt

import (
	uv "github.com/charmbracelet/ultraviolet"
	"strings"
)

type logicalRow struct {
	cells         uv.Line
	cursor, saved int
}
type reflowRow struct {
	cells uv.Line
	wrap  int
}
type reflowPosition struct {
	x, y    int
	phantom bool
}

func usedColumns(line uv.Line) int {
	end := 0
	for x, c := range line {
		if !c.IsZero() && !c.Equal(&uv.EmptyCell) {
			end = max(end, x+max(1, c.Width))
		}
	}
	return min(end, len(line))
}
func (s *Screen) logicalRows(phantom bool) []logicalRow {
	history := s.scrollback.Len()
	last := max(s.cur.Y, s.saved.Y)
	for y := s.Height() - 1; y > last; y-- {
		if usedColumns(s.buf.Line(y)) > 0 {
			last = y
			break
		}
	}
	rows := []logicalRow{}
	continued := false
	for i := 0; i < history+last+1; i++ {
		var line uv.Line
		wrap := 0
		if i < history {
			line = s.scrollback.Line(i)
			wrap = s.scrollback.WrappedAt(i)
		} else {
			line = s.buf.Line(i - history)
			wrap = s.wrapAt[i-history]
		}
		if !continued {
			rows = append(rows, logicalRow{cursor: -1, saved: -1})
		}
		row := &rows[len(rows)-1]
		used := usedColumns(line)
		if wrap > 0 {
			used = min(wrap, len(line))
		}
		if i == history+s.cur.Y {
			x := s.cur.X
			if phantom {
				x++
			}
			row.cursor = len(row.cells) + x
			used = max(used, x)
		}
		if i == history+s.saved.Y {
			row.saved = len(row.cells) + s.saved.X
			used = max(used, s.saved.X)
		}
		if !continued && wrap == 0 {
			row.cells = line[:min(used, len(line))]
		} else {
			row.cells = append(row.cells, line[:min(used, len(line))]...)
		}
		continued = wrap > 0
	}
	return rows
}
func (s *Screen) reflowResize(width, height int, phantom bool) bool {
	groups := s.logicalRows(phantom)
	rows := []reflowRow{}
	cursor, saved := reflowPosition{}, reflowPosition{}
	for _, group := range groups {
		line := make(uv.Line, 0, min(width, len(group.cells)))
		x := 0
		locate := func(offset, at, span int) {
			if offset >= at && offset < at+span {
				pos := reflowPosition{x: x + offset - at, y: len(rows)}
				if offset == group.cursor {
					cursor = pos
				}
				if offset == group.saved {
					saved = pos
				}
			}
		}
		for i := 0; i < len(group.cells); {
			cell := group.cells[i]
			span := max(1, cell.Width)
			if cell.Width == 0 {
				cell = uv.EmptyCell
			}
			if x+span > width {
				rows = append(rows, reflowRow{line, x})
				line = make(uv.Line, 0, min(width, len(group.cells)-i))
				x = 0
			}
			locate(group.cursor, i, span)
			locate(group.saved, i, span)
			for range span {
				line = append(line, uv.Cell{})
			}
			line[x] = cell
			x += span
			i += span
			if x == width && i < len(group.cells) {
				rows = append(rows, reflowRow{line, x})
				line = make(uv.Line, 0, min(width, len(group.cells)-i))
				x = 0
			}
		}
		end := reflowPosition{x: min(x, width-1), y: len(rows), phantom: x == width}
		if group.cursor == len(group.cells) {
			cursor = end
		}
		if group.saved == len(group.cells) {
			saved = end
		}
		rows = append(rows, reflowRow{cells: line})
	}
	start := max(0, len(rows)-height)
	if cursor.y < start {
		start = max(0, cursor.y)
	}
	if s.scrollback != nil {
		s.scrollback.Clear()
		for _, row := range rows[:start] {
			s.scrollback.pushWrapped(row.cells, row.wrap)
		}
	}
	s.buf = uv.NewRenderBuffer(width, height)
	s.wrapAt = make([]int, height)
	for y, row := range rows[start:min(len(rows), start+height)] {
		for x := 0; x < len(row.cells); {
			cell := row.cells[x]
			if cell.Width > 0 {
				s.buf.SetCell(x, y, &cell)
			}
			x += max(1, cell.Width)
		}
		s.wrapAt[y] = row.wrap
	}
	s.scroll = s.buf.Bounds()
	s.setCursor(cursor.x, max(0, min(height-1, cursor.y-start)), false)
	s.saved.X, s.saved.Y = saved.x, max(0, min(height-1, saved.y-start))
	return cursor.phantom
}

// LogicalLines reconstructs output lines by joining terminal-generated wraps.
// Explicit newlines remain separate; cell display width preserves table gaps.
func (e *Emulator) LogicalLines() []string {
	groups := e.scr.logicalRows(e.atPhantom)
	out := make([]string, 0, len(groups))
	for _, row := range groups {
		var b strings.Builder
		for x := 0; x < len(row.cells); {
			cell := row.cells[x]
			if cell.Content == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(cell.Content)
			}
			x += max(1, cell.Width)
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}
