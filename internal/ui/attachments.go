package ui

import (
	"fmt"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
)

type attachmentComposer struct {
	revision           uint64
	input              *chatInput
	area               fyne.CanvasObject
	rows               *fyne.Container
	explicit, detected []string
	dismissed          map[string]bool
}

func (u *App) newAttachmentComposer(input *chatInput) *attachmentComposer {
	c := &attachmentComposer{input: input, rows: container.NewVBox(), dismissed: map[string]bool{}}
	input.PasteFiles = platforminput.ClipboardFiles
	input.OnFiles = func(paths []string) {
		if err := c.add(paths); err != nil {
			u.error(err)
		}
	}
	input.OnPasteError = u.error
	input.OnChanged = func(text string) {
		c.detected = localfiles.References(text)
		for p := range c.dismissed {
			if !slices.Contains(c.detected, p) {
				delete(c.dismissed, p)
			}
		}
		c.refresh()
	}
	return c
}
func (c *attachmentComposer) paths() []string {
	var out []string
	seen := map[string]bool{}
	for _, paths := range [][]string{c.explicit, c.detected} {
		for _, p := range paths {
			if !seen[p] && !c.dismissed[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}
func (c *attachmentComposer) add(paths []string) error {
	var validated []string
	for _, p := range paths {
		real, err := localfiles.ResolveReference(p)
		if err != nil {
			return fmt.Errorf("无法添加文件 %s：%w", p, err)
		}
		validated = append(validated, real)
	}
	for _, p := range validated {
		delete(c.dismissed, p)
		c.explicit = append(c.explicit, p)
	}
	c.refresh()
	return nil
}
func (c *attachmentComposer) refresh() {
	c.revision++
	c.rows.RemoveAll()
	for _, p := range c.paths() {
		label := widget.NewLabel(filepath.Base(p))
		label.Truncation = fyne.TextTruncateEllipsis
		remove := action("", designIcon("close"), func() { c.remove(p) })
		c.rows.Add(container.NewBorder(nil, nil, widget.NewIcon(theme.DocumentIcon()), remove, label))
	}
	if len(c.rows.Objects) == 0 {
		c.rows.Hide()
	} else {
		c.rows.Show()
	}
}
func (c *attachmentComposer) remove(p string) {
	c.explicit = slices.DeleteFunc(c.explicit, func(value string) bool { return value == p })
	if slices.Contains(c.detected, p) {
		c.dismissed[p] = true
	} else {
		delete(c.dismissed, p)
	}
	c.refresh()
}
func (c *attachmentComposer) clear() {
	c.explicit = nil
	c.detected = nil
	c.dismissed = map[string]bool{}
	c.input.SetText("")
	c.refresh()
}
func (u *App) chooseAttachment() {
	dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			u.error(err)
			return
		}
		if reader == nil {
			return
		}
		uri := reader.URI()
		reader.Close()
		if uri.Scheme() != "file" {
			u.error(fmt.Errorf("请选择本机文件"))
			return
		}
		if err = u.composer.add([]string{uri.Path()}); err != nil {
			u.error(err)
		}
	}, u.Window).Show()
}
func (u *App) dropAttachments(pos fyne.Position, uris []fyne.URI) bool {
	c := u.composer
	if c == nil || c.area == nil || !u.assistantVisible {
		return false
	}
	origin := u.UI.Driver().AbsolutePositionForObject(c.area)
	if !containsPosition(pos, origin, c.area.Size()) {
		return false
	}
	var paths []string
	for _, uri := range uris {
		if uri.Scheme() != "file" {
			u.error(fmt.Errorf("请选择本机文件"))
			return true
		}
		paths = append(paths, uri.Path())
	}
	if err := c.add(paths); err != nil {
		u.error(err)
	}
	return true
}
