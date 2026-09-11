package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"os"
	"path"
	"path/filepath"
	"time"
)

func containsPosition(pos, origin fyne.Position, size fyne.Size) bool {
	return pos.X >= origin.X && pos.Y >= origin.Y && pos.X < origin.X+size.Width && pos.Y < origin.Y+size.Height
}

func (u *App) nativeFilesDropped(pos fyne.Position, uris []fyne.URI) {
	// Fyne coalesces mouse movement until the next frame. Read the native
	// pointer at drop time, before dispatching UI work, rather than its old position.
	if window, ok := u.Window.(driver.NativeWindow); ok {
		window.RunNative(func(c any) {
			var handle uintptr
			switch c := c.(type) {
			case driver.MacWindowContext:
				handle = c.NSWindow
			case driver.WindowsWindowContext:
				handle = c.HWND
			}
			if x, y, ok := platforminput.DropPosition(handle); ok {
				size := u.Window.Canvas().Size()
				pos = fyne.NewPos(float32(x)*size.Width, float32(y)*size.Height)
			}
		})
	}
	fyne.Do(func() { u.filesDropped(pos, uris) })
}

func (u *App) filesDropped(pos fyne.Position, uris []fyne.URI) {
	if u.closing || u.filePickerOpen || len(uris) == 0 || len(u.Window.Canvas().Overlays().List()) > 0 {
		return
	}
	if u.dropAttachments(pos, uris) {
		return
	}
	for _, w := range u.workspaces {
		if w.ctx.Err() != nil || u.tabs.Selected() != w.tab {
			continue
		}
		origin := u.UI.Driver().AbsolutePositionForObject(w.tab.Content)
		if !containsPosition(pos, origin, w.tab.Content.Size()) {
			continue
		}
		paths := []string{}
		for _, uri := range uris {
			if uri.Scheme() != "file" {
				u.error(fmt.Errorf("只能上传本机文件"))
				return
			}
			paths = append(paths, uri.Path())
		}
		origin = u.UI.Driver().AbsolutePositionForObject(w.fileArea)
		if w.bottomTabs.Selected() == w.fileTab && containsPosition(pos, origin, w.fileArea.Size()) {
			w.queueUploads(paths, w.dir.Text)
		} else {
			session := w.activeSession.Load()
			for i, terminal := range w.terminals {
				origin := u.UI.Driver().AbsolutePositionForObject(terminal)
				if containsPosition(pos, origin, terminal.Size()) && i < len(w.sessions) {
					session = w.sessions[i]
					break
				}
			}
			if session == nil {
				u.error(fmt.Errorf("终端尚未连接，请连接后再上传文件"))
				return
			}
			// Resolve the dropped-on terminal's cwd, even if the file browser
			// has navigated elsewhere or another split pane has keyboard focus.
			u.work("准备上传", func() error {
				directory, err := u.Manager.TerminalDirectory(w.ctx, w.host.ID, session)
				if err != nil {
					return err
				}
				fyne.Do(func() {
					if w.ctx.Err() == nil {
						w.queueUploads(paths, directory)
					}
				})
				return nil
			})
		}
		return
	}
}
func (w *workspace) queueUploads(locals []string, directory string) {
	if len(locals) == 0 || w.ctx.Err() != nil {
		return
	}
	if !path.IsAbs(directory) {
		w.u.error(fmt.Errorf("请等待终端目录就绪或选择上传目录"))
		return
	}
	// Capture the destination now, so later cd, tab switches or file navigation cannot retarget uploads.
	for _, local := range locals {
		info, err := os.Stat(local)
		if err != nil {
			w.u.error(err)
			return
		}
		if !info.Mode().IsRegular() {
			w.u.error(fmt.Errorf("%s 不是普通文件，请先将目录打包", filepath.Base(local)))
			return
		}
	}
	for _, local := range locals {
		dest := path.Join(directory, filepath.Base(local))
		w.u.work("准备上传", func() error {
			c, err := w.u.Manager.SFTP(w.ctx, w.host.ID)
			if err != nil {
				return err
			}
			defer c.Close()
			info, err := c.Lstat(dest)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			exists := err == nil
			if exists && !info.Mode().IsRegular() {
				return fmt.Errorf("目标 %s 不是普通文件", dest)
			}
			fyne.Do(func() {
				if w.ctx.Err() != nil {
					return
				}
				if !exists {
					w.transfer(local, dest, true, false, true)
					return
				}
				var d dialog.Dialog
				d = newMotionDialog("目标文件已存在", "取消", container.NewVBox(widget.NewLabel(dest), container.NewHBox(widget.NewButton("校验并续传", func() { d.Hide(); w.transfer(local, dest, true, true) }), widget.NewButton("覆盖上传", func() { d.Hide(); w.transfer(local, dest, true, false) }))), w.u.Window)
				d.Show()
			})
			return nil
		})
	}
}
func (w *workspace) watchDirectory() {
	var previousSession *remote.TerminalSession
	previousDir := ""
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		session := w.activeSession.Load()
		if session != nil {
			dir, err := w.u.Manager.TerminalDirectory(w.ctx, w.host.ID, session)
			changed := session != previousSession || dir != previousDir
			fyne.Do(func() {
				if w.ctx.Err() != nil || w.activeSession.Load() != session {
					return
				}
				if err != nil {
					w.directoryStatus.SetText("终端目录不可用")
					return
				}
				w.directoryStatus.SetText("")
				if w.dir.Text == "" || (w.followDirectory.Checked && changed) {
					w.dir.SetText(dir)
					w.refreshFiles()
				}
			})
			if err == nil {
				previousSession = session
				previousDir = dir
			}
		}
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
