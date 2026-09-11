package ui

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

type resizeCounter struct {
	*canvas.Rectangle
	calls int
}

func (r *resizeCounter) Resize(size fyne.Size) { r.calls++; r.Rectangle.Resize(size) }

func TestAssistantSlideInterruptResizeAndReducedMotion(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("assistant motion")
	defer w.Close()
	u := &App{UI: a, Window: w, assistantVisible: true, leftWidth: 240, rightWidth: 340}
	box := func() fyne.CanvasObject { return canvas.NewRectangle(color.White) }
	center := &resizeCounter{Rectangle: canvas.NewRectangle(color.Black)}
	u.desktopBody = container.New(workbenchLayout{u: u}, box(), center, box(), box(), box())
	w.SetContent(u.desktopBody)
	w.Resize(fyne.NewSize(1400, 800))
	w.Show()
	u.assistantSlide = newAssistantSlide(u)
	m := u.assistantSlide
	m.enabled = func() bool { return true }
	var frames []*fyne.Animation
	m.startAnimation = func(a *fyne.Animation) { frames = append(frames, a) }
	panel, divider := u.desktopBody.Objects[2], u.desktopBody.Objects[4]
	width, rest := panel.Size().Width, panel.Position()
	aligned := func() {
		t.Helper()
		want := panel.Position().X - dividerSize
		if m.reveal == 0 {
			want = u.desktopBody.Size().Width
		}
		if math.Abs(float64(center.Position().X+center.Size().Width-want)) > .01 {
			t.Fatal("center edge did not move together with the assistant")
		}
	}
	initialCenter := center.Size()
	u.toggleAssistant()
	if center.Size() != initialCenter {
		t.Fatal("center jumped to final width at transition start")
	}
	if !panel.Visible() || u.assistantVisible || divider.Visible() {
		t.Fatal("closing panel disappeared before its slide")
	}
	if len(frames) != 1 || frames[0].Duration != popupExitDuration {
		t.Fatal("wrong exit timing")
	}
	resizes := center.calls
	frames[0].Tick(.4)
	if panel.Position().X <= rest.X || panel.Size().Width != width || center.calls <= resizes {
		t.Fatal("center did not resize together with the sliding panel")
	}
	aligned()
	partial := panel.Position()
	u.toggleAssistant()
	if panel.Position() != partial {
		t.Fatal("quick reversal jumped back to an endpoint")
	}
	frames[0].Tick(1)
	if panel.Position() != partial || !panel.Visible() {
		t.Fatal("stale close frame hid the reopened panel")
	}
	if frames[1].Duration != popupEnterDuration {
		t.Fatal("wrong entry timing")
	}
	frames[1].Tick(.5)
	aligned()
	if panel.Position().X >= partial.X {
		t.Fatal("reopen did not reverse direction")
	}
	w.Resize(fyne.NewSize(1200, 700))
	u.desktopBody.Refresh()
	aligned()
	frames[1].Tick(1)
	if !panel.Visible() || !divider.Visible() || panel.Position().X+panel.Size().Width != u.desktopBody.Size().Width {
		t.Fatal("resize during transition lost the right edge")
	}
	if u.rightWidth != 340 {
		t.Fatal("animation changed saved panel width")
	}
	u.toggleAssistant()
	frames[2].Tick(1)
	if panel.Visible() || divider.Visible() || center.Position().X+center.Size().Width != u.desktopBody.Size().Width {
		t.Fatal("closed state did not release the pane space")
	}
	u.toggleAssistant()
	frames[3].Tick(.3)
	m.enabled = func() bool { return false }
	frames[3].Tick(.4)
	if m.animation != nil || m.reveal != 1 || !divider.Visible() {
		t.Fatal("reduced motion did not settle the current transition")
	}
	u.toggleAssistant()
	if panel.Visible() || len(frames) != 4 {
		t.Fatal("disabled motion started another animation")
	}
}

func TestAssistantSlideIgnoresFramesAfterStop(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	u := &App{assistantVisible: true, leftWidth: 240, rightWidth: 340}
	box := func() fyne.CanvasObject { return canvas.NewRectangle(color.White) }
	u.desktopBody = container.New(workbenchLayout{u: u}, box(), box(), box(), box(), box())
	u.desktopBody.Resize(fyne.NewSize(1400, 800))
	u.assistantSlide = newAssistantSlide(u)
	u.assistantSlide.enabled = func() bool { return true }
	var frame *fyne.Animation
	u.assistantSlide.startAnimation = func(a *fyne.Animation) { frame = a }
	u.toggleAssistant()
	u.assistantSlide.stop()
	before := u.desktopBody.Objects[2].Position()
	frame.Tick(1)
	if u.desktopBody.Objects[2].Position() != before {
		t.Fatal("stopped animation changed detached content")
	}
}
