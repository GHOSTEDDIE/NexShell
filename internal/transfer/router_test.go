package transfer

import (
	"bytes"
	"context"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer/zmodem"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZmodemReturnsToTerminal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	a.SetDeadline(time.Now().Add(5 * time.Second))
	b.SetDeadline(time.Now().Add(5 * time.Second))
	src, dst := t.TempDir(), t.TempDir()
	file := filepath.Join(src, "payload")
	data := bytes.Repeat([]byte("data\x18\x11\x00"), 500)
	os.WriteFile(file, data, 0600)
	r := NewRouter(ctx, a, a, Hooks{DownloadDirectory: func(context.Context) (string, error) { return dst, nil }})
	defer r.Close()
	sent := make(chan error, 1)
	go func() {
		c := zmodem.New(b, b)
		if e := c.WriteHeader(zmodem.Position(zmodem.ZRQINIT, 0)); e != nil {
			sent <- e
			return
		}
		e := c.Send(ctx, []string{file}, nil, nil)
		if e == nil {
			_, e = io.WriteString(b, "\r\nshell$ ")
		}
		sent <- e
	}()
	var output strings.Builder
	buf := make([]byte, 256)
	for !strings.Contains(output.String(), "shell$") {
		n, e := r.Read(buf)
		if e != nil {
			t.Fatal(e)
		}
		output.Write(buf[:n])
	}
	if e := <-sent; e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(filepath.Join(dst, "payload"))
	if e != nil || !bytes.Equal(data, got) {
		t.Fatal("payload mismatch", e)
	}
	if strings.Contains(output.String(), "B000") {
		t.Fatal("protocol leaked into terminal")
	}
}
