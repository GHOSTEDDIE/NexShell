package ui

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer"
	"strings"
)

func (w *workspace) newTerminal(session *remote.TerminalSession) *terminal.View {
	router := transfer.NewRouter(w.ctx, session.Input, session.Output, transfer.Hooks{
		UploadFiles: func(ctx context.Context) ([]string, error) {
			type choice struct {
				files []string
				err   error
			}
			ch := make(chan choice, 1)
			fyne.Do(func() {
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, e error) {
					if e != nil {
						ch <- choice{err: e}
						return
					}
					if r == nil {
						ch <- choice{err: errors.New("已取消上传")}
						return
					}
					p := r.URI().Path()
					r.Close()
					ch <- choice{files: []string{p}}
				}, w.u.Window)
			})
			select {
			case c := <-ch:
				return c.files, c.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		DownloadDirectory: func(ctx context.Context) (string, error) {
			type choice struct {
				dir string
				err error
			}
			ch := make(chan choice, 1)
			fyne.Do(func() {
				dialog.ShowFolderOpen(func(uri fyne.ListableURI, e error) {
					if e != nil {
						ch <- choice{err: e}
						return
					}
					if uri == nil {
						ch <- choice{err: errors.New("已取消下载")}
						return
					}
					ch <- choice{dir: uri.Path()}
				}, w.u.Window)
			})
			select {
			case c := <-ch:
				return c.dir, c.err
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
		Progress: func(name string, n, total int64) {
			fyne.Do(func() { w.u.status.SetText(fmt.Sprintf("ZMODEM · %s · %d / %d 字节", name, n, total)) })
		},
		State: func(active bool, err error) {
			fyne.Do(func() {
				if active {
					w.u.status.SetText("ZMODEM 传输中，按 Ctrl+C 取消")
				} else if err != nil {
					w.u.status.SetText("ZMODEM：" + err.Error())
				} else {
					w.u.status.SetText("ZMODEM 传输完成")
				}
			})
		},
	})
	w.routers = append(w.routers, router)
	view := terminal.NewView(router, router, func(err error) { w.u.status.SetText(w.host.Name + "：" + err.Error()) })
	id := w.u.terminalsDesktop.Register(w.host.ID, router, func() string { return strings.Join(view.Core.Lines(), "\n") }, func() bool {
		if w.ctx.Err() != nil {
			return false
		}
		select {
		case <-session.Done:
			return false
		default:
			return true
		}
	})
	view.SetFontSize(float32(w.u.UI.Preferences().FloatWithFallback("terminal.size", float64(terminal.DefaultFontSize))))
	w.u.applyTerminalBackground(view)
	view.OnFocus = func() { w.u.terminalsDesktop.Select(id); w.activeSession.Store(session) }
	w.terminalIDs = append(w.terminalIDs, id)
	return view
}
