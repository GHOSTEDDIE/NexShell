package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"os"
	"path/filepath"
	"testing"
)

func TestComposerPathPasteAndAttachmentRemoval(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	p := filepath.Join(t.TempDir(), "部署包.tar.gz")
	if err := os.WriteFile(p, []byte{0, 255}, 0600); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(p)
	u := &App{UI: app}
	input := newChatInput()
	c := u.newAttachmentComposer(input)
	input.SetText("上传 `" + p + "` 到服务器")
	if len(c.paths()) != 1 || c.paths()[0] != real {
		t.Fatal("text path not attached", c.paths())
	}
	c.clear()
	input.SetText("部署这个包")
	input.PasteFiles = func() ([]string, error) { return []string{p}, nil }
	clip := test.NewClipboard()
	clip.SetContent("should not replace draft")
	input.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clip})
	if input.Text != "部署这个包" || len(c.paths()) != 1 {
		t.Fatal("file paste lost draft or attachment")
	}
	// Pasting a normal message must continue through Fyne's existing text editor.
	input.PasteFiles = func() ([]string, error) { return nil, nil }
	input.SetText("")
	clip.SetContent("继续聊天")
	input.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clip})
	if input.Text != "继续聊天" {
		t.Fatal("text paste broken", input.Text)
	}
	c.refresh()
	row := c.rows.Objects[0].(*fyne.Container)
	var remove *actionButton
	for _, o := range row.Objects {
		if button, ok := o.(*actionButton); ok {
			remove = button
		}
	}
	if remove == nil {
		t.Fatal("remove button missing")
	}
	test.Tap(remove)
	if len(c.paths()) != 0 {
		t.Fatal("removed file still attached")
	}
	input.SetText("读取 `" + p + "`")
	if len(c.paths()) != 1 {
		t.Fatal("a newly typed path stayed dismissed")
	}
	c.remove(real)
	if len(c.paths()) != 0 {
		t.Fatal("text attachment removal ignored")
	}
	input.SetText("")
	input.SetText(p)
	if len(c.paths()) != 1 {
		t.Fatal("path could not be re-added")
	}
	c.clear()
	if err := c.add([]string{p, p}); err != nil || len(c.paths()) != 1 {
		t.Fatal("duplicate attachment", err)
	}
	c.area = container.NewStack(input)
	c.area.Resize(fyne.NewSize(200, 100))
	u.composer = c
	u.assistantVisible = true
}
