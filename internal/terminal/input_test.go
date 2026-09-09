package terminal

import (
	"bytes"
	"errors"
	uv "github.com/charmbracelet/ultraviolet"
	"io"
	"strings"
	"testing"
	"time"
)

func TestEncodedInputOrderAndProtocolReplies(t *testing.T) {
	c := NewCore(80, 24)
	defer c.CloseInput()
	c.Write([]byte("\x1b[?2004h"))
	c.Text("中文\n", true)
	c.Key(uv.Key{Code: uv.KeyEnter})
	c.Write([]byte("\x1b[6n"))
	want := []byte("\x1b[200~中文\n\x1b[201~\r\x1b[1;1R")
	got := make([]byte, len(want))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded input changed: %q", got)
	}
}
func TestInputOverflowAndCloseReleaseReaders(t *testing.T) {
	p := newInputBuffer()
	if _, err := p.Write(bytes.Repeat([]byte{'x'}, maxPendingInput)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("x")); !errors.Is(err, errInputBacklog) {
		t.Fatal("missing overflow error", err)
	}
	if p.data.Len() != 0 {
		t.Fatal("backlog retained")
	}
	if _, err := p.Read(make([]byte, 1)); !errors.Is(err, errInputBacklog) {
		t.Fatal(err)
	}
	p = newInputBuffer()
	done := make(chan error, 1)
	go func() { _, err := p.Read(make([]byte, 1)); done <- err }()
	p.Close()
	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reader leaked")
	}
	if _, err := p.Write([]byte("late")); err == nil {
		t.Fatal("closed pipe accepted input")
	}
}
func TestScrollbackWrapPreservesSearchAndAlternateScreen(t *testing.T) {
	c := NewCore(30, 3)
	defer c.CloseInput()
	for i := 0; i < 10020; i++ {
		c.Write([]byte("中文保留\r\n"))
	}
	if c.Snapshot(0).History != 10000 {
		t.Fatal("history limit changed")
	}
	if len(c.Search("中文保留")) != 10002 {
		t.Fatal("wrapped search lost lines", len(c.Search("中文保留")))
	}
	before := strings.Join(c.Lines(), "\n")
	c.Write([]byte("\x1b[?1049halternate\x1b[?1049l"))
	c.Resize(40, 4)
	if !strings.Contains(strings.Join(c.Lines(), "\n"), "中文保留") || !strings.HasPrefix(before, "中文保留") {
		t.Fatal("history changed after alternate screen or resize")
	}
}
