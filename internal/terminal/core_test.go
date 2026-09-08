package terminal

import (
	"io"
	"strings"
	"testing"
)

func coreForTest(t *testing.T, cols, rows int) *Core {
	c := NewCore(cols, rows)
	go io.Copy(io.Discard, c)
	t.Cleanup(func() { c.CloseInput() })
	return c
}
func TestUTF8AndAlternateScreen(t *testing.T) {
	c := coreForTest(t, 20, 4)
	c.Write([]byte("健康OK"))
	s := c.Snapshot(0)
	if s.Rows[0][0].Text != "健" || s.Rows[0][0].Width != 2 || s.CursorX != 6 {
		t.Fatalf("wide cells: %+v", s)
	}
	c.Write([]byte("\x1b[?1049hTEMP\x1b[?1049l"))
	s = c.Snapshot(0)
	if s.Alt || s.Rows[0][0].Text != "健" {
		t.Fatal("main screen lost")
	}
	c.Resize(40, 8)
	if len(c.Snapshot(0).Rows) != 8 {
		t.Fatal("resize failed")
	}
}
func TestScrollbackAndSearch(t *testing.T) {
	c := coreForTest(t, 30, 3)
	for _, s := range []string{"first\r\n", "second\r\n", "third\r\n", "fourth\r\n"} {
		c.Write([]byte(s))
	}
	if c.Snapshot(0).History < 2 {
		t.Fatal("scrollback lost")
	}
	if len(c.Search("first")) != 1 {
		t.Fatal("historical search failed")
	}
	if !strings.Contains(strings.Join(c.Lines(), "\n"), "fourth") {
		t.Fatal("visible screen lost")
	}
}
func TestUTF8Fragmentation(t *testing.T) {
	c := coreForTest(t, 20, 2)
	for _, b := range []byte("中文") {
		c.Write([]byte{b})
	}
	if c.Snapshot(0).CursorX != 4 {
		t.Fatal("split UTF8 broke width")
	}
}

func TestCombiningCharacterAcrossWrites(t *testing.T) {
	c := coreForTest(t, 20, 2)
	c.Write([]byte("e"))
	c.Write([]byte("\u0301"))
	s := c.Snapshot(0)
	if s.CursorX != 1 || s.Rows[0][0].Text != "e\u0301" {
		t.Fatalf("combined cell not retained: %+v", s.Rows[0][:3])
	}
}
