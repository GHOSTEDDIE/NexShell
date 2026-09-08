package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"image/png"
	"os"
	"testing"
)

func TestDesktopLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	m, e := remote.NewManager(s, store.Credentials{}, s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	executor := &remote.Executor{Manager: m, Store: s}
	agents := agent.NewService(s, executor, store.Credentials{})
	u := New(app, s, m, executor, agents)
	defer func() {
		u.cancel()
		agents.Close()
		m.Close()
		s.Close()
		u.Window.SetCloseIntercept(nil)
		u.Window.Close()
	}()
	u.Window.Resize(fyne.NewSize(1380, 900))
	u.Window.Show()
	if u.taskList.PlaceHolder != "选择任务" {
		t.Fatal("task selector is not localized")
	}
	if p := os.Getenv("MYAIT_SCREENSHOT"); p != "" {
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, u.Window.Canvas().Capture()); e != nil {
			t.Fatal(e)
		}
	}
}
