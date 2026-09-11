package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"testing"
)

func TestSettingsNavigationAligned(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := remote.NewManager(s, store.Credentials{}, s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	ex := &remote.Executor{Store: s, Manager: m}
	ag := agent.NewService(s, ex, store.Credentials{})
	defer ag.Close()
	u := New(a, s, m, ex, ag)
	defer func() { u.cancel(); u.Window.SetCloseIntercept(nil); u.Window.Close() }()
	u.Window.Resize(fyne.NewSize(1200, 800))
	u.Window.Show()
	u.settingsDialog("appearance")
	var nav []*actionButton
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *actionButton:
			if v.Text == "外观与显示" || v.Text == "模型配置" {
				nav = append(nav, v)
			}
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *surface:
			walk(v.Content)
		}
	}
	walk(u.settingsPopup.Content)
	if len(nav) != 2 {
		t.Fatalf("found %d navigation buttons", len(nav))
	}
	r1 := test.WidgetRenderer(nav[0]).(*actionRenderer)
	r2 := test.WidgetRenderer(nav[1]).(*actionRenderer)
	if r1.icon.Position().X != r2.icon.Position().X || r1.text.Position().X != r2.text.Position().X {
		t.Fatalf("navigation columns misaligned: icons %.1f/%.1f text %.1f/%.1f", r1.icon.Position().X, r2.icon.Position().X, r1.text.Position().X, r2.text.Position().X)
	}
	u.settingsDialog("models")
	var check func(fyne.CanvasObject)
	count := 0
	check = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *actionButton:
			if v.Text == "保存" || v.Text == "取消" {
				count++
				if v.Size().Height > 32 || v.Size().Width < 72 {
					t.Errorf("footer %s has unbalanced size %v", v.Text, v.Size())
				}
			}
		case *fyne.Container:
			for _, child := range v.Objects {
				check(child)
			}
		case *surface:
			check(v.Content)
		}
	}
	check(u.settingsPopup.Content)
	if count != 2 {
		t.Fatalf("expected two footer controls, found %d", count)
	}

}
