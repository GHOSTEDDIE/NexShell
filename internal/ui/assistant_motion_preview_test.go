package ui

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
)

func TestAssistantMotionPreview(t *testing.T) {
	dir := os.Getenv("NEXSHELL_MOTION_EVIDENCE")
	if dir == "" {
		t.Skip("optional rendered motion evidence")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	a := test.NewApp()
	defer a.Quit()
	if os.Getenv("NEXSHELL_STYLE_DARK") == "1" {
		setAppearance(a, true)
	}
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	manager, err := remote.NewManager(s, store.Credentials{}, s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ex := &remote.Executor{Store: s, Manager: manager}
	ag := agent.NewService(s, ex, store.Credentials{})
	defer ag.Close()
	u := New(a, s, manager, ex, ag)
	defer func() { u.cancel(); u.Window.SetCloseIntercept(nil); u.Window.Close() }()
	v := terminal.NewView(io.Discard, strings.NewReader(""))
	v.Close()
	v.Core.Write([]byte("ops@app-01:~$ docker ps\r\nCONTAINER ID   IMAGE          STATUS          NAMES\r\n28a7246fb019   nginx:alpine   Up 12 days      web\r\nops@app-01:~$ "))
	ctx, cancel := context.WithCancel(u.ctx)
	defer cancel()
	ws := &workspace{u: u, ctx: ctx, cancel: cancel, host: domain.Host{ID: "motion", Name: "测试服务器", User: "ops", Address: "192.0.2.10"}, terminal: v, terminals: []*terminal.View{v}, monitor: widget.NewLabel("CPU 12% · 内存 38%")}
	ws.layoutWorkspace()
	u.showWorkspace(ws)
	u.Window.Resize(fyne.NewSize(1440, 940))
	u.Window.Show()
	u.assistantSlide = newAssistantSlide(u)
	u.assistantSlide.enabled = func() bool { return true }
	var animation *fyne.Animation
	u.assistantSlide.startAnimation = func(a *fyne.Animation) { animation = a }
	save := func(n int) {
		f, e := os.Create(filepath.Join(dir, fmt.Sprintf("frame-%02d.png", n)))
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, u.Window.Canvas().Capture()); e != nil {
			t.Fatal(e)
		}
	}
	originalLines := v.Core.LogicalLines()
	save(0)
	u.toggleAssistant()
	for i := 1; i <= 8; i++ {
		animation.Tick(animation.Curve(float32(i) / 8))
		if ws.bottomTabs.Size().Width != ws.tab.Content.Size().Width {
			t.Fatal("bottom pane did not resize with terminal workspace")
		}
		save(i)
	}
	u.toggleAssistant()
	for i := 1; i <= 8; i++ {
		animation.Tick(animation.Curve(float32(i) / 8))
		if ws.bottomTabs.Size().Width != ws.tab.Content.Size().Width {
			t.Fatal("bottom pane did not resize with terminal workspace")
		}
		save(i + 8)
	}
	if !reflect.DeepEqual(originalLines, v.Core.LogicalLines()) {
		t.Fatal("synchronized resize changed terminal output")
	}
	if os.Getenv("NEXSHELL_STYLE_PREVIEW") == "1" {
		u.settingsDialog("appearance")
		save(17)
		u.settingsDialog("models")
		save(18)
	}
}

func BenchmarkAssistantLayoutWithHistory(b *testing.B) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	win := a.NewWindow("motion benchmark")
	defer win.Close()
	v := terminal.NewView(io.Discard, strings.NewReader(""))
	v.Close()
	u := &App{UI: a, Window: win, assistantVisible: true, leftWidth: 240, rightWidth: 340}
	box := func() fyne.CanvasObject { return widget.NewLabel("") }
	u.desktopBody = container.New(workbenchLayout{u: u}, box(), v, box(), box(), box())
	win.SetContent(u.desktopBody)
	win.Resize(fyne.NewSize(1400, 800))
	win.Show()
	u.assistantSlide = newAssistantSlide(u)
	v.Core.Write([]byte(strings.Repeat("ops@app-01:~$ abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\r\n", 2000)))
	positions := []float32{0, .25, .5, .75, 1, .75, .5, .25}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.assistantSlide.reveal = positions[i%len(positions)]
		u.desktopBody.Layout.Layout(u.desktopBody.Objects, u.desktopBody.Size())
	}
}
