package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
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
	if os.Getenv("MYAIT_DARK") == "1" {
		setAppearance(app, true)
	}
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

	u.tabs.CloseIntercept(u.homeTab)
	if len(u.tabs.Items) != 1 {
		t.Fatal("connection homepage was closed")
	}
	if u.homeList.Length() != 0 {
		t.Fatal("home should initially be empty")
	}
	h := domain.Host{ID: "home-test", Name: "测试服务器", Address: "192.0.2.10", Port: 22, User: "ops", Group: "测试环境"}
	if err := s.Put("hosts", h.ID, h); err != nil {
		t.Fatal(err)
	}
	u.refreshHosts()
	if u.homeList.Length() != 1 || u.homeCount.Text != "已保存连接 · 1" {
		t.Fatal("home did not refresh saved connections")
	}
	u.search.SetText("not-found")
	if u.homeList.Length() != 0 {
		t.Fatal("homepage search did not filter connections")
	}
	u.search.SetText("")
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
