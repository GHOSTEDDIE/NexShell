package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"math"
)

const dividerSize float32 = 8
const minimumLeftWidth float32 = 200
const minimumCenterWidth float32 = 480
const minimumRightWidth float32 = 280
const minimumTerminalHeight float32 = 180
const minimumToolsHeight float32 = 150

type resizeDivider struct {
	widget.BaseWidget
	horizontal        bool
	hovered, dragging bool
	change            func(float32)
	save              func()
}

func newResizeDivider(horizontal bool, change func(float32), save func()) *resizeDivider {
	d := &resizeDivider{horizontal: horizontal, change: change, save: save}
	d.ExtendBaseWidget(d)
	return d
}
func (d *resizeDivider) Cursor() desktop.Cursor {
	if d.horizontal {
		return desktop.VResizeCursor
	}
	return desktop.HResizeCursor
}
func (d *resizeDivider) MouseIn(*desktop.MouseEvent)    { d.hovered = true; d.Refresh() }
func (d *resizeDivider) MouseMoved(*desktop.MouseEvent) {}
func (d *resizeDivider) MouseOut()                      { d.hovered = false; d.Refresh() }
func (d *resizeDivider) Dragged(e *fyne.DragEvent) {
	d.dragging = true
	delta := e.Dragged.DX
	if d.horizontal {
		delta = e.Dragged.DY
	}
	d.change(delta)
	d.Refresh()
}
func (d *resizeDivider) DragEnd() {
	d.dragging = false
	if d.save != nil {
		d.save()
	}
	d.Refresh()
}
func (d *resizeDivider) CreateRenderer() fyne.WidgetRenderer {
	r := &dividerRenderer{d: d, line: canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))}
	r.Refresh()
	return r
}

type dividerRenderer struct {
	d    *resizeDivider
	line *canvas.Rectangle
}

func (r *dividerRenderer) MinSize() fyne.Size {
	if r.d.horizontal {
		return fyne.NewSize(0, dividerSize)
	}
	return fyne.NewSize(dividerSize, 0)
}
func (r *dividerRenderer) Layout(s fyne.Size) {
	thickness := float32(1)
	if r.d.hovered || r.d.dragging {
		thickness = 2
	}
	if r.d.horizontal {
		r.line.Move(fyne.NewPos(0, (s.Height-thickness)/2))
		r.line.Resize(fyne.NewSize(s.Width, thickness))
	} else {
		r.line.Move(fyne.NewPos((s.Width-thickness)/2, 0))
		r.line.Resize(fyne.NewSize(thickness, s.Height))
	}
}
func (r *dividerRenderer) Refresh() {
	r.line.FillColor = theme.Color(theme.ColorNameSeparator)
	if r.d.hovered || r.d.dragging {
		r.line.FillColor = theme.Color(theme.ColorNamePrimary)
	}
	r.Layout(r.d.Size())
	r.line.Refresh()
}
func (r *dividerRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.line} }
func (r *dividerRenderer) Destroy()                     {}

func savedPanelSize(p fyne.Preferences, key string, fallback float32) float32 {
	v := p.FloatWithFallback(key, float64(fallback))
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 10000 {
		return fallback
	}
	return float32(v)
}
func boundedPanelSize(value, minimum, maximum float32) float32 {
	return min(max(0, maximum), max(minimum, value))
}

// Keep both side panes stable while there is enough room. When shrinking the
// main window, share the shortage without overlapping the central terminal.
func fitWorkbenchWidths(width, left, right float32, assistant bool) (float32, float32, float32) {
	gutters := dividerSize
	minimumRight := float32(0)
	if assistant {
		gutters += dividerSize
		minimumRight = minimumRightWidth
		right = max(minimumRight, right)
	} else {
		right = 0
	}
	available := max(0, width-gutters)
	left = max(minimumLeftWidth, left)
	budget := max(0, available-minimumCenterWidth)
	if left+right > budget {
		minimumSides := minimumLeftWidth + minimumRight
		if budget < minimumSides {
			left = budget * minimumLeftWidth / minimumSides
			right = budget - left
		} else {
			flex := left + right - minimumSides
			scale := float32(0)
			if flex > 0 {
				scale = (budget - minimumSides) / flex
			}
			left = minimumLeftWidth + (left-minimumLeftWidth)*scale
			right = minimumRight + (right-minimumRight)*scale
		}
	}
	return left, max(0, available-left-right), right
}
