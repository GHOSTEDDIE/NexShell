package ui

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"github.com/GHOSTEDDIE/nexshell/internal/nativefiles"
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
				w.u.pickFiles(ctx, nativefiles.Request{Mode: nativefiles.OpenMultiple, Title: "选择上传文件"}, func(paths []string, err error) {
					if err == nil && len(paths) == 0 {
						err = errors.New("已取消上传")
					}
					ch <- choice{files: paths, err: err}
				})
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
				w.u.pickFiles(ctx, nativefiles.Request{Mode: nativefiles.Directory, Title: "选择下载目录"}, func(paths []string, err error) {
					if err == nil && len(paths) == 0 {
						err = errors.New("已取消下载")
					}
					c := choice{err: err}
					if len(paths) > 0 {
						c.dir = paths[0]
					}
					ch <- c
				})
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
