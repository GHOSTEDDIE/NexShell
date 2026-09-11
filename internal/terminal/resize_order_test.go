package terminal

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLatestResizeWinsAfterSlowRemote(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(readabilityTheme{theme.DefaultTheme()})
	v := NewView(io.Discard, strings.NewReader(""))
	defer v.Close()
	v.dirty.Store(false)
	started := make(chan struct{})
	release := make(chan struct{})
	newStarted := make(chan struct{})
	finished := make(chan int, 2)
	var calls atomic.Int32
	v.OnResize = func(rows, cols int) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		} else {
			close(newStarted)
		}
		finished <- rows
	}
	v.Resize(fyne.NewSize(800, 400))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first resize not sent")
	}
	v.Resize(fyne.NewSize(800, 600))
	want := v.rows
	select {
	case <-newStarted:
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	last := 0
	for i := 0; i < 2; i++ {
		select {
		case last = <-finished:
		case <-time.After(time.Second):
			t.Fatal("latest resize not delivered")
		}
	}
	if last != want {
		t.Fatalf("slow old resize overwrote latest geometry: got %d rows, want %d", last, want)
	}
}
