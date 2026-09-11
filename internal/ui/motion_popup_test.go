package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestPopupMotionLifecycle(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	w := a.NewWindow("motion")
	w.Resize(fyne.NewSize(900, 600))
	w.Show()
	defer w.Close()
	p := newMotionPopup(widget.NewLabel("content"), w.Canvas())
	p.Resize(fyne.NewSize(400, 200))
	p.enabled = func() bool { return true }
	var frames []*fyne.Animation
	p.startAnimation = func(a *fyne.Animation) { frames = append(frames, a) }
	p.Show()
	rest := p.restingPosition()
	if p.Position().Y != rest.Y+18 {
		t.Fatal("missing entry offset")
	}
	frames[0].Tick(.5)
	if p.Position().Y <= rest.Y || p.Position().Y >= rest.Y+18 {
		t.Fatal("entry frame did not interpolate")
	}
	calls := 0
	p.close(func() { calls++ })
	p.close(func() { calls++ })
	if len(frames) != 2 {
		t.Fatal("duplicate close started extra animation")
	}
	beforeExit := p.Position().Y
	frames[1].Tick(.5)
	if p.Position().Y <= beforeExit {
		t.Fatal("exit reversed toward the entering position")
	}
	frames[0].Tick(1)
	if !p.Visible() || calls != 0 {
		t.Fatal("stale entry callback closed popup")
	}
	frames[1].Tick(1)
	if p.Visible() || calls != 1 || len(w.Canvas().Overlays().List()) != 0 {
		t.Fatal("close did not release overlay once")
	}
	p.Show()
	p.close(func() { calls++ })
	exit := frames[len(frames)-1]
	p.Show()
	exit.Tick(1)
	if !p.Visible() || calls != 1 {
		t.Fatal("stale exit hid reopened popup")
	}
	p.enabled = func() bool { return false }
	p.Hide()
	if p.Visible() {
		t.Fatal("disabled motion did not hide immediately")
	}
}
func TestMotionFormRetainsValidationAndSingleSubmit(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	w := a.NewWindow("form")
	w.Resize(fyne.NewSize(900, 600))
	w.Show()
	defer w.Close()
	entry := widget.NewEntry()
	entry.Validator = func(s string) error {
		if s != "ok" {
			return fmt.Errorf("invalid")
		}
		return nil
	}
	responses := 0
	closed := 0
	d := newMotionForm("编辑", "确认", "取消", []*widget.FormItem{widget.NewFormItem("名称", entry)}, func(ok bool) {
		if !ok {
			t.Error("wrong response")
		}
		responses++
	}, w)
	d.SetOnClosed(func() { closed++ })
	d.popup.enabled = func() bool { return false }
	d.Show()
	test.Tap(d.confirm)
	if responses != 0 {
		t.Fatal("invalid form submitted")
	}
	entry.SetText("ok")
	test.Tap(d.confirm)
	test.Tap(d.confirm)
	if responses != 1 || closed != 1 {
		t.Fatal("submission/closed callback duplicated")
	}
}
