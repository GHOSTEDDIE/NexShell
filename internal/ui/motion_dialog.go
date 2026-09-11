package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Retain Fyne's form validation and the Dialog API while sharing one popup
// transition and one compact action row across application dialogs.
type motionDialog struct {
	popup            *motionPopup
	dismiss, confirm *actionButton
	callback         func(bool)
	onClosed         []func()
	closing          bool
	validate         func() error
}

var _ dialog.Dialog = (*motionDialog)(nil)

func (d *motionDialog) Show() {
	d.closing = false
	d.dismiss.Enable()
	if d.confirm != nil {
		if d.validate != nil && d.validate() != nil {
			d.confirm.Disable()
		} else {
			d.confirm.Enable()
		}
	}
	if d.popup.Size().IsZero() {
		size := d.MinSize().Max(fyne.NewSize(420, 180))
		canvasSize := d.popup.Canvas.Size()
		size.Width = min(size.Width, max(1, canvasSize.Width-40))
		size.Height = min(size.Height, max(1, canvasSize.Height-40))
		d.popup.Resize(size)
	}
	d.popup.Show()
}

func (d *motionDialog) Hide()                       { d.finish(false) }
func (d *motionDialog) Dismiss()                    { d.Hide() }
func (d *motionDialog) Refresh()                    { d.popup.Refresh() }
func (d *motionDialog) Resize(size fyne.Size)       { d.popup.Resize(size.Max(d.MinSize())) }
func (d *motionDialog) MinSize() fyne.Size          { return d.popup.MinSize() }
func (d *motionDialog) SetDismissText(label string) { d.dismiss.SetText(label) }
func (d *motionDialog) SetOnClosed(fn func())       { d.onClosed = append(d.onClosed, fn) }
func (d *motionDialog) finish(accepted bool) {
	if d.closing {
		return
	}
	d.closing = true
	d.dismiss.Disable()
	if d.confirm != nil {
		d.confirm.Disable()
	}
	d.popup.close(func() {
		if d.callback != nil {
			d.callback(accepted)
		}
		for _, fn := range d.onClosed {
			fn()
		}
	})
}
func newMotionConfirm(title, confirm, dismiss string, content fyne.CanvasObject, callback func(bool), parent fyne.Window) *motionDialog {
	d := &motionDialog{callback: callback}
	d.dismiss = outlineAction(dismiss, nil, func() { d.finish(false) })
	d.dismiss.minWidth = 72
	actions := container.NewHBox(d.dismiss)
	if confirm != "" {
		d.confirm = action(confirm, nil, func() { d.finish(true) })
		d.confirm.minWidth = 72
		d.confirm.primary = true
		actions.Add(d.confirm)
	}
	head := inset(container.NewBorder(nil, nil, headingText(title), action("", designIcon("close"), d.Hide)), 12, 20, 12, 20)
	footer := inset(container.NewBorder(nil, nil, nil, container.NewCenter(actions), layout.NewSpacer()), 10, 20, 10, 20)
	body := edge(panel(head, theme.ColorNameBackground, true, 0), panel(footer, colorSoft, true, 0), nil, nil, inset(content, 12, 20, 16, 20))
	d.popup = newMotionPopup(panel(body, theme.ColorNameBackground, false, 0), parent.Canvas())
	return d
}
func newMotionDialog(title, dismiss string, content fyne.CanvasObject, parent fyne.Window) *motionDialog {
	return newMotionConfirm(title, "", dismiss, content, nil, parent)
}
func showMotionDialog(title, dismiss string, content fyne.CanvasObject, parent fyne.Window) {
	newMotionDialog(title, dismiss, content, parent).Show()
}
func showMotionConfirm(title, message string, callback func(bool), parent fyne.Window) {
	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord
	newMotionConfirm(title, "确认", "取消", label, callback, parent).Show()
}
func showMotionInformation(title, message string, parent fyne.Window) {
	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord
	showMotionDialog(title, "关闭", label, parent)
}
func showMotionError(err error, parent fyne.Window) {
	showMotionInformation("操作未完成", err.Error(), parent)
}
func newMotionForm(title, confirm, dismiss string, items []*widget.FormItem, callback func(bool), parent fyne.Window) *motionDialog {
	form := widget.NewForm(items...)
	d := newMotionConfirm(title, confirm, dismiss, form, callback, parent)
	d.validate = form.Validate
	state := func(err error) {
		if err != nil || d.closing {
			d.confirm.Disable()
		} else {
			d.confirm.Enable()
		}
	}
	state(form.Validate())
	form.SetOnValidationChanged(state)
	return d
}
func showMotionForm(title, confirm, dismiss string, items []*widget.FormItem, callback func(bool), parent fyne.Window) {
	newMotionForm(title, confirm, dismiss, items, callback, parent).Show()
}
