package ui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbenchPanelDragging(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	manager, e := remote.NewManager(s, store.Credentials{}, s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer manager.Close()
	ex := &remote.Executor{Store: s, Manager: manager}
	ag := agent.NewService(s, ex, store.Credentials{})
	defer ag.Close()
	u := New(a, s, manager, ex, ag)
	defer func() { u.cancel(); u.Window.SetCloseIntercept(nil); u.Window.Close() }()
	v := terminal.NewView(io.Discard, strings.NewReader(""))
	v.Close()
	ctx, cancel := context.WithCancel(u.ctx)
	defer cancel()
	w := &workspace{u: u, host: domain.Host{ID: "resize", Name: "拖拽测试", User: "test", Address: "127.0.0.1"}, terminal: v, terminals: []*terminal.View{v}, ctx: ctx, cancel: cancel, monitor: widget.NewLabel("CPU 12% · 内存 38%")}
	w.layoutWorkspace()
	u.showWorkspace(w)
	u.Window.Resize(fyne.NewSize(1440, 940))
	u.Window.Show()
	u.desktopBody.Refresh()
	drag := func(o fyne.CanvasObject, dx, dy float32) {
		t.Helper()
		point := a.Driver().AbsolutePositionForObject(o).Add(fyne.NewPos(o.Size().Width/2, o.Size().Height/2))
		test.Drag(u.Window.Canvas(), point, dx, dy)
	}
	near := func(got, want float32) {
		t.Helper()
		if math.Abs(float64(got-want)) > .01 {
			t.Fatalf("size %.2f, want %.2f", got, want)
		}
	}
	left, right := u.desktopBody.Objects[0], u.desktopBody.Objects[2]
	beforeLeft, beforeRight := left.Size().Width, right.Size().Width
	drag(u.desktopBody.Objects[3], 70, 0)
	near(left.Size().Width, beforeLeft+70)
	near(right.Size().Width, beforeRight)
	drag(u.desktopBody.Objects[4], -50, 0)
	near(right.Size().Width, beforeRight+50)
	near(left.Size().Width, beforeLeft+70)
	near(savedPanelSize(a.Preferences(), "layout.leftWidth", 0), left.Size().Width)
	near(savedPanelSize(a.Preferences(), "layout.rightWidth", 0), right.Size().Width)
	content := w.tab.Content.(*fyne.Container)
	beforeBottom := w.bottomTabs.Size().Height
	drag(content.Objects[2], 0, -80)
	near(w.bottomTabs.Size().Height, beforeBottom+80)
	near(savedPanelSize(a.Preferences(), "layout.bottomHeight", 0), beforeBottom+80)
	savedRight := right.Size().Width
	u.toggleAssistant()
	if u.desktopBody.Objects[4].Visible() {
		t.Fatal("hidden assistant left an active divider")
	}
	near(u.tabs.Position().X+u.tabs.Size().Width, u.desktopBody.Size().Width)
	u.toggleAssistant()
	near(right.Size().Width, savedRight)
	if dir := os.Getenv("NEXSHELL_RESIZE_EVIDENCE"); dir != "" {
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		f, e := os.Create(filepath.Join(dir, "resized-panels.png"))
		if e != nil {
			t.Fatal(e)
		}
		e = png.Encode(f, u.Window.Canvas().Capture())
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	drag(u.desktopBody.Objects[3], -10000, 0)
	near(left.Size().Width, minimumLeftWidth)
	drag(u.desktopBody.Objects[4], -10000, 0)
	near(u.tabs.Size().Width, minimumCenterWidth)
	drag(content.Objects[2], 0, 10000)
	near(w.bottomTabs.Size().Height, minimumToolsHeight)
	u.Window.Resize(fyne.NewSize(1100, 700))
	u.desktopBody.Refresh()
	near(left.Size().Width+u.tabs.Size().Width+right.Size().Width+2*dividerSize, u.desktopBody.Size().Width)
	if left.Size().Width < minimumLeftWidth || right.Size().Width < minimumRightWidth || u.tabs.Size().Width < minimumCenterWidth {
		t.Fatal("window resize violated pane minimums")
	}
	// Existing session tabs follow the last saved bottom height when selected.
	w.bottomHeight = 250
	u.selectWorkspace(w.tab)
	near(w.bottomHeight, minimumToolsHeight)
}
