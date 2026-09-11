package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"runtime"
)

func (u *App) configureLifecycle() {
	u.Window.SetCloseIntercept(u.closeWindow)
	if runtime.GOOS == "darwin" {
		u.UI.Lifecycle().SetOnStarted(func() {
			fyne.Do(func() {
				if err := platforminput.InstallApplicationActions(u.restoreWindow, u.quit); err != nil {
					u.error(err)
				}
			})
		})
	}
}
func (u *App) closeWindow() {
	if u.closing {
		return
	}
	if runtime.GOOS == "darwin" {
		u.Window.Hide()
		return
	}
	u.quit()
}
func (u *App) restoreWindow() {
	if u.closing {
		return
	}
	u.Window.Show()
	u.Window.RequestFocus()
}

// Explicit quit owns shutdown; hiding a macOS window keeps this state alive.
func (u *App) quit() {
	if u.closing {
		return
	}
	u.closing = true
	u.status.SetText("正在保存任务与关闭连接…")
	u.cancel()
	u.Agent.Stop()
	workspaces := u.workspaces
	u.workspaces = nil
	tunnels := append([]*remote.Tunnel(nil), u.tunnels...)
	u.Window.SetContent(container.NewCenter(widget.NewLabel("正在保存任务与关闭连接…")))
	go func() {
		u.Manager.Close()
		for _, ws := range workspaces {
			ws.close()
		}
		for _, t := range tunnels {
			t.Close()
		}
		u.Agent.Close()
		_ = u.Store.Close()
		fyne.Do(func() { u.Window.SetCloseIntercept(nil); u.Window.Close(); u.UI.Quit() })
	}()
}
