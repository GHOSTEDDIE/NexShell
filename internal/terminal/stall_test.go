package terminal

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"image/color"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type auditBlockedWriter struct {
	entered, release chan struct{}
	once             sync.Once
}

func (w *auditBlockedWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

func TestAuditSnapshotDuringStalledPaste(t *testing.T) {
	c := NewCore(80, 24)
	w := &auditBlockedWriter{entered: make(chan struct{}), release: make(chan struct{})}
	copyDone := make(chan struct{})
	go func() { io.Copy(w, c); close(copyDone) }()
	sent := make(chan struct{})
	go func() { c.Text(strings.Repeat("x", 65536), true); close(sent) }()
	snapshot := make(chan struct{})
	snapshotStarted := false
	defer func() {
		c.CloseInput()
		close(w.release)
		<-sent
		<-copyDone
		if snapshotStarted {
			<-snapshot
		}
	}()
	select {
	case <-w.entered:
	case <-time.After(time.Second):
		t.Fatal("paste did not reach network writer")
	}
	snapshotStarted = true
	go func() {
		// All UI-facing state operations must remain independent of SSH writes.
		c.Snapshot(0)
		c.Resize(100, 30)
		c.MouseTracking()
		c.Write([]byte("\x1b[6n"))
		close(snapshot)
	}()

	// Cleanup must release the writer before joining Snapshot.
	select {
	case <-snapshot:
	case <-time.After(150 * time.Millisecond):
		t.Error("UI Snapshot remains blocked while network writer stalls")
	}
}

func BenchmarkAuditFullScrollback(b *testing.B) {
	for _, limit := range []int{1000, 10000} {
		b.Run(fmt.Sprint(limit), func(b *testing.B) {
			s := vt.NewScrollback(limit)
			line := uv.Line{{Content: "x", Width: 1}}
			for i := 0; i < limit; i++ {
				s.Push(line)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Push(line)
			}
		})
	}
}

type auditTheme struct{ fyne.Theme }

func (t auditTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == "terminalBackground" {
		return color.Black
	}
	if n == "terminalForeground" {
		return color.White
	}
	return t.Theme.Color(n, v)
}
func BenchmarkAuditTerminalPaint(b *testing.B) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(auditTheme{a.Settings().Theme()})
	v := &View{Core: NewCore(120, 35), fontSize: 14, cols: 120, rows: 35, selectionStart: -1, selectionEnd: -1}
	v.ExtendBaseWidget(v)
	v.measure()
	v.Core.Write([]byte(strings.Repeat("sample log line abcdefghijklmnopqrstuvwxyz\r\n", 35)))
	r := v.CreateRenderer().(*renderer)
	r.paint()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.paint()
	}
}

type failedInputWriter struct{}

func (failedInputWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestFailedInputStopsPipeAndSubsequentKeys(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(auditTheme{a.Settings().Theme()})
	reported := make(chan error, 1)
	v := NewView(failedInputWriter{}, strings.NewReader(""), func(err error) {
		select {
		case reported <- err:
		default:
		}
	})
	defer v.Close()
	v.Send("first")
	select {
	case err := <-reported:
		if err != io.ErrClosedPipe {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("input failure not reported")
	}
	if v.Core.input.Err() != io.ErrClosedPipe {
		t.Fatal("failed input pipe still open")
	}
	done := make(chan struct{})
	go func() { v.Core.Text("next", false); v.Core.Snapshot(0); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("input failure blocked subsequent state access")
	}
}
