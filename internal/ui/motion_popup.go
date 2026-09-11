package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
	"time"
)

const popupEnterDuration = 180 * time.Millisecond
const popupExitDuration = 120 * time.Millisecond

func motionEnabled() bool {
	app := fyne.CurrentApp()
	return app != nil && app.Settings().ShowAnimations() && app.Preferences().BoolWithFallback("appearance.motion", true) && !platforminput.ReducedMotion()
}

// Move the complete popup without resizing text or delaying form validation.
// A generation invalidates queued frames when an animation is interrupted.
type motionPopup struct {
	*widget.PopUp
	animation      *fyne.Animation
	generation     uint64
	closing        bool
	enabled        func() bool
	startAnimation func(*fyne.Animation)
}

func newMotionPopup(content fyne.CanvasObject, canvas fyne.Canvas) *motionPopup {
	return &motionPopup{PopUp: widget.NewModalPopUp(content, canvas), enabled: motionEnabled, startAnimation: func(a *fyne.Animation) { a.Start() }}
}
func (p *motionPopup) stop() {
	p.generation++
	if p.animation != nil {
		p.animation.Stop()
		p.animation = nil
	}
}
func (p *motionPopup) restingPosition() fyne.Position {
	s := p.Canvas.Size()
	return fyne.NewPos((s.Width-p.Size().Width)/2, (s.Height-p.Size().Height)/2)
}
func (p *motionPopup) Show() {
	p.stop()
	p.closing = false
	p.PopUp.Show()
	if !p.enabled() {
		return
	}
	p.animate(false, nil)
}
func (p *motionPopup) Hide() { p.close(nil) }
func (p *motionPopup) close(done func()) {
	if p.closing {
		return
	}
	p.stop()
	p.closing = true
	finish := func() {
		p.PopUp.Hide()
		p.closing = false
		if done != nil {
			done()
		}
	}
	if !p.Visible() || !p.enabled() {
		finish()
		return
	}
	p.animate(true, finish)
}
func (p *motionPopup) animate(exit bool, done func()) {
	generation := p.generation
	duration := popupEnterDuration
	startOffset := float32(18)
	endOffset := float32(0)
	if exit {
		duration = popupExitDuration
		startOffset = p.Position().Y - p.restingPosition().Y
		endOffset = max(12, startOffset+8)
	}
	move := func(progress float32) {
		if generation != p.generation {
			return
		}
		position := p.restingPosition()
		position.Y += startOffset + (endOffset-startOffset)*progress
		p.Move(position)
		if progress >= 1 {
			p.animation = nil
			if done != nil {
				done()
			}
		}
	}
	move(0)
	animation := fyne.NewAnimation(duration, func(progress float32) { fyne.Do(func() { move(progress) }) })
	animation.Curve = func(t float32) float32 { v := 1 - t; return 1 - v*v*v }
	p.animation = animation
	p.startAnimation(animation)
}

func (p *motionPopup) hideImmediately() { p.stop(); p.PopUp.Hide(); p.closing = false }
