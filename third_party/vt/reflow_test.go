package vt

import (
	"fmt"
	uv "github.com/charmbracelet/ultraviolet"
	"reflect"
	"strings"
	"testing"
)

func lineText(line uv.Line) string {
	var b strings.Builder
	for _, c := range line {
		if c.Width > 0 {
			b.WriteString(c.Content)
		}
	}
	return strings.TrimRight(b.String(), " ")
}
func primaryText(e *Emulator) string {
	var b strings.Builder
	for _, l := range e.Scrollback().Lines() {
		b.WriteString(lineText(l))
	}
	for y := 0; y < e.Height(); y++ {
		b.WriteString(lineText(e.scr.buf.Line(y)))
	}
	return b.String()
}
func TestResizeReflowsWithoutLosingOutput(t *testing.T) {
	e := NewEmulator(40, 6)
	defer e.Close()
	e.Write([]byte("abcdefghijklmnopqrstuvwxyz0123456789\r\nnext"))
	want := primaryText(e)
	e.Resize(12, 6)
	if got := primaryText(e); got != want {
		t.Fatalf("shrink lost output: %q, want %q", got, want)
	}
	e.Resize(40, 6)
	if got := lineText(e.scr.buf.Line(0)); got != "abcdefghijklmnopqrstuvwxyz0123456789" {
		t.Fatalf("expand did not unwrap: %q", got)
	}
	e.Write([]byte("!"))
	if !strings.Contains(primaryText(e), "next!") {
		t.Fatal("cursor did not follow reflow")
	}
}
func TestResizeJoinsSoftWrapButPreservesNewline(t *testing.T) {
	e := NewEmulator(8, 6)
	defer e.Close()
	e.Write([]byte("abcdefghijkl\r\nnext"))
	e.Resize(24, 6)
	if lineText(e.scr.buf.Line(0)) != "abcdefghijkl" || lineText(e.scr.buf.Line(1)) != "next" {
		t.Fatalf("soft/hard breaks not preserved: %q / %q", lineText(e.scr.buf.Line(0)), lineText(e.scr.buf.Line(1)))
	}
}
func TestHeightShrinkKeepsOutputInHistory(t *testing.T) {
	e := NewEmulator(20, 8)
	defer e.Close()
	e.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\nfive\r\nsix"))
	want := primaryText(e)
	e.Resize(20, 3)
	if got := primaryText(e); got != want {
		t.Fatalf("height shrink lost output: %q", got)
	}
}

func TestUnicodeAndStyledReflow(t *testing.T) {
	e := NewEmulator(7, 4)
	defer e.Close()
	text := "你好世界e\u0301👩‍💻abcdefghijklmnop"
	e.Write([]byte("\x1b[31m" + text + "\x1b[0m\r\nprompt> "))
	want := e.LogicalLines()
	for _, width := range []int{5, 40, 9, 80} {
		e.Resize(width, 5)
		if got := e.LogicalLines(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Unicode/hard breaks damaged at %d: %q want %q", width, got, want)
		}
		found := false
		for _, row := range e.scr.logicalRows(e.atPhantom) {
			for _, cell := range row.cells {
				if cell.Content == "你" && cell.Style.Fg != nil {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("ANSI style lost")
		}
	}
}
func TestHistoryReflowAndAlternateScreen(t *testing.T) {
	e := NewEmulator(12, 5)
	defer e.Close()
	for i := 0; i < 50; i++ {
		e.Write([]byte(fmt.Sprintf("row-%02d-long-content\r\n", i)))
	}
	want := e.LogicalLines()
	e.Resize(40, 8)
	if !reflect.DeepEqual(want, e.LogicalLines()) {
		t.Fatal("scrollback reflow changed logical history")
	}
	e.Write([]byte("\x1b[?1049h"))
	e.Write([]byte("top\x1b[3;1Hbottom"))
	e.Resize(18, 6)
	if !e.IsAltScreen() || lineText(e.scr.buf.Line(0)) != "top" || lineText(e.scr.buf.Line(2)) != "bottom" {
		t.Fatal("alternate screen was reflowed")
	}
}

func TestAlternateResizeRestoresPrimaryAndPendingWrap(t *testing.T) {
	e := NewEmulator(8, 4)
	defer e.Close()
	e.Write([]byte("abcdefghijklmnop"))
	want := e.LogicalLines()
	e.Write([]byte("\x1b[?1049h"))
	e.Resize(6, 4)
	e.Write([]byte("full screen"))
	e.Write([]byte("\x1b[?1049l"))
	if !reflect.DeepEqual(want, e.LogicalLines()) {
		t.Fatal("resizing full-screen app destroyed shell output")
	}
	e.Write([]byte("!"))
	if e.LogicalLines()[0] != "abcdefghijklmnop!" {
		t.Fatalf("restored cursor overwrote output: %q", e.LogicalLines())
	}
}
func BenchmarkResizeReflowHistory(b *testing.B) {
	e := NewEmulator(100, 30)
	defer e.Close()
	for i := 0; i < 2000; i++ {
		e.Write([]byte(fmt.Sprintf("row-%04d-abcdefghijklmnopqrstuvwxyz0123456789-abcdefghijklmnop\r\n", i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Resize(60+(i%2)*60, 30)
	}
}
