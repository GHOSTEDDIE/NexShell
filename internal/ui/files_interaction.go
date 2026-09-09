package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"os"
	"path"
	"path/filepath"
	"time"
)

func containsPosition(pos, origin fyne.Position, size fyne.Size) bool {
	return pos.X >= origin.X && pos.Y >= origin.Y && pos.X < origin.X+size.Width && pos.Y < origin.Y+size.Height
}
func (u *App) filesDropped(pos fyne.Position, uris []fyne.URI) {
	if u.closing {
		return
	}
	for _, w := range u.workspaces {
		if w.ctx.Err() != nil || u.tabs.Selected() != w.tab || w.bottomTabs.Selected() != w.fileTab {
			continue
		}
		origin := u.UI.Driver().AbsolutePositionForObject(w.fileArea)
		if !containsPosition(pos, origin, w.fileArea.Size()) {
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
		w.queueUploads(paths, w.dir.Text)
		return
	}
}
func (w *workspace) queueUploads(locals []string, directory string) {
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
				d = dialog.NewCustom("目标文件已存在", "取消", container.NewVBox(widget.NewLabel(dest), container.NewHBox(widget.NewButton("校验并续传", func() { d.Hide(); w.transfer(local, dest, true, true) }), widget.NewButton("覆盖上传", func() { d.Hide(); w.transfer(local, dest, true, false) }))), w.u.Window)
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
