package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// readOnly retains selection and copy with normal contrast, while refusing edits.
type readOnly struct{ widget.Entry }

func newReadOnly() *readOnly {
	e := &readOnly{}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}
func (e *readOnly) TypedRune(rune)          {}
func (e *readOnly) TypedKey(*fyne.KeyEvent) {}
func (e *readOnly) TypedShortcut(s fyne.Shortcut) {
	switch s.(type) {
	case *fyne.ShortcutCopy, *fyne.ShortcutSelectAll:
		e.Entry.TypedShortcut(s)
	}
}
