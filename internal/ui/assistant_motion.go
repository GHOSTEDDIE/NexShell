package ui

import "fyne.io/fyne/v2"

// Keep the assistant at its full reading width. One reveal value drives both
// its translation and the adjacent workbench width so their edges stay joined.
type assistantSlide struct {
	u              *App
	reveal         float32
	generation     uint64
	animation      *fyne.Animation
	enabled        func() bool
	startAnimation func(*fyne.Animation)
}

func newAssistantSlide(u *App) *assistantSlide {
	m := &assistantSlide{u: u, enabled: motionEnabled, startAnimation: func(a *fyne.Animation) { a.Start() }}
	if u.assistantVisible {
		m.reveal = 1
	}
	return m
}

func (m *assistantSlide) stop() {
	m.generation++
	if m.animation != nil {
		m.animation.Stop()
		m.animation = nil
	}
}

func (m *assistantSlide) transition() {
	m.stop()
	target := float32(0)
	duration := popupExitDuration
	if m.u.assistantVisible {
		target = 1
		duration = popupEnterDuration
	}
	if !m.enabled() || m.reveal == target {
		m.reveal = target
		m.u.desktopBody.Refresh()
		return
	}
	from, generation := m.reveal, m.generation
	a := fyne.NewAnimation(duration, func(progress float32) {
		fyne.Do(func() {
			if generation != m.generation || m.u.closing {
				return
			}
			if !m.enabled() {
				progress = 1
			}
			m.reveal = from + (target-from)*progress
			if progress >= 1 {
				m.stop()
				m.reveal = target
			}
			// Relayout only: refreshing the whole container would also repaint
			// every unaffected widget for each animation frame.
			body := m.u.desktopBody
			body.Layout.Layout(body.Objects, body.Size())
		})
	})
	a.Curve = motionEaseOut
	m.animation = a
	m.u.desktopBody.Refresh()
	m.startAnimation(a)
}

func (u *App) assistantReveal() float32 {
	reveal := float32(0)
	if u.assistantVisible {
		reveal = 1
	}
	if u.assistantSlide != nil {
		reveal = u.assistantSlide.reveal
	}
	return reveal
}

func (u *App) positionAssistant() {
	if u.desktopBody == nil {
		return
	}
	panel, divider := u.desktopBody.Objects[2], u.desktopBody.Objects[4]
	reveal := u.assistantReveal()
	if reveal == 0 {
		panel.Hide()
		divider.Hide()
		return
	}
	x := u.desktopBody.Size().Width - panel.Size().Width + (panel.Size().Width+dividerSize)*(1-reveal)
	panel.Move(fyne.NewPos(x, 0))
	panel.Show()
	divider.Move(fyne.NewPos(x-dividerSize, 0))
	if u.assistantVisible && reveal == 1 {
		divider.Show()
	} else {
		divider.Hide()
	}
}
