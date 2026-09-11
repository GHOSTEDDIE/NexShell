package ui

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
)

type lifecycleTestApp struct {
	fyne.App
	done chan struct{}
	once sync.Once
}

func (a *lifecycleTestApp) Quit() { a.once.Do(func() { a.App.Quit(); close(a.done) }) }

type lifecycleTestWindow struct {
	fyne.Window
	hidden, shown, focused int
	closeRequested         func()
}

func (w *lifecycleTestWindow) Hide()         { w.hidden++; w.Window.Hide() }
func (w *lifecycleTestWindow) Show()         { w.shown++; w.Window.Show() }
func (w *lifecycleTestWindow) RequestFocus() { w.focused++; w.Window.RequestFocus() }
func (w *lifecycleTestWindow) SetCloseIntercept(fn func()) {
	w.closeRequested = fn
	w.Window.SetCloseIntercept(fn)
}
func lifecycleFixture(t *testing.T) (*App, *lifecycleTestWindow, *lifecycleTestApp) {
	t.Helper()
	a := &lifecycleTestApp{App: test.NewApp(), done: make(chan struct{})}
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := remote.NewManager(s, store.Credentials{}, s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	e := &remote.Executor{Manager: m, Store: s}
	agents := agent.NewService(s, e, store.Credentials{})
	u := New(a, s, m, e, agents)
	w := &lifecycleTestWindow{Window: u.Window}
	u.Window = w
	u.configureLifecycle()
	t.Cleanup(func() {
		if !u.closing {
			u.quit()
		}
		select {
		case <-a.done:
		case <-time.After(5 * time.Second):
			t.Error("shutdown did not finish")
		}
	})
	return u, w, a
}
func TestMacWindowCloseKeepsApplicationAlive(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS window behavior")
	}
	u, w, a := lifecycleFixture(t)
	task := domain.Task{ID: "keep", Status: "running"}
	if err := u.Store.Put("tasks", task.ID, task); err != nil {
		t.Fatal(err)
	}
	w.closeRequested()
	if u.closing || u.ctx.Err() != nil || w.hidden != 1 {
		t.Fatal("window close shut down the application")
	}
	select {
	case <-a.done:
		t.Fatal("application quit on window close")
	default:
	}
	var stored domain.Task
	if err := u.Store.Load("tasks", task.ID, &stored); err != nil || stored.Status != "running" {
		t.Fatal("task state lost", err)
	}
	u.restoreWindow()
	if w.shown != 1 || w.focused != 1 {
		t.Fatal("reopen did not restore and focus the window")
	}
	w.closeRequested()
	u.restoreWindow()
	if w.hidden != 2 || w.shown != 2 {
		t.Fatal("repeated close/reopen failed")
	}
}
func TestExplicitQuitUsesCleanupOnce(t *testing.T) {
	u, w, a := lifecycleFixture(t)
	var quit *fyne.MenuItem
	for _, menu := range w.MainMenu().Items {
		for _, item := range menu.Items {
			if item.IsQuit {
				quit = item
			}
		}
	}
	if quit == nil || quit.Action == nil {
		t.Fatal("explicit quit action missing")
	}
	quit.Action()
	quit.Action()
	select {
	case <-a.done:
	case <-time.After(5 * time.Second):
		t.Fatal("explicit quit blocked")
	}
	if u.ctx.Err() != context.Canceled {
		t.Fatal("application workers not canceled")
	}
	var value any
	if err := u.Store.Load("tasks", "any", &value); err == nil {
		t.Fatal("store remained open")
	}
	shown := w.shown
	u.restoreWindow()
	if w.shown != shown {
		t.Fatal("quitting application reopened")
	}
}
