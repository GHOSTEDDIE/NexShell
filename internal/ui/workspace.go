package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type workspace struct {
	u            *App
	host         domain.Host
	session      *remote.TerminalSession
	terminal     *terminal.View
	terminals    []*terminal.View
	sessions     []*remote.TerminalSession
	tab          *container.TabItem
	ctx          context.Context
	cancel       context.CancelFunc
	once         sync.Once
	files        []remote.FileEntry
	fileList     *widget.List
	dir          *widget.Entry
	selectedFile int
	transfers    *fyne.Container
	routers      []*transfer.Router
}

func (u *App) newWorkspace(h domain.Host, s *remote.TerminalSession) *workspace {
	ctx, cancel := context.WithCancel(u.ctx)
	w := &workspace{u: u, host: h, session: s, ctx: ctx, cancel: cancel, selectedFile: -1}
	w.terminal = w.newTerminal(s)
	w.terminal.OnResize = func(rows, cols int) { _ = s.Resize(rows, cols) }
	w.terminals = []*terminal.View{w.terminal}
	w.sessions = []*remote.TerminalSession{s}
	find := widget.NewEntry()
	find.SetPlaceHolder("搜索终端输出")
	find.OnSubmitted = func(q string) { count := w.terminal.Search(q); u.status.SetText(fmt.Sprintf("找到 %d 行", count)) }
	holder := container.NewStack(w.terminal)
	toolbar := container.NewHBox(widget.NewLabel(h.User+"@"+h.Address), widget.NewButton("分屏", func() {
		u.work("创建分屏", func() error {
			next, e := u.Manager.Terminal(ctx, h.ID, 24, 80)
			if e != nil {
				return e
			}
			fyne.Do(func() {
				v := w.newTerminal(next)
				v.OnResize = func(r, c int) { _ = next.Resize(r, c) }
				w.terminals = append(w.terminals, v)
				w.sessions = append(w.sessions, next)
				old := holder.Objects[0]
				holder.Objects = []fyne.CanvasObject{container.NewHSplit(old, v)}
				holder.Refresh()
			})
			return nil
		})
	}), widget.NewButton("A−", func() {
		w.terminal.SetFontSize(float32(u.UI.Preferences().FloatWithFallback("terminal.size", 14) - 1))
		u.UI.Preferences().SetFloat("terminal.size", u.UI.Preferences().FloatWithFallback("terminal.size", 14)-1)
	}), widget.NewButton("A+", func() {
		size := u.UI.Preferences().FloatWithFallback("terminal.size", 14) + 1
		u.UI.Preferences().SetFloat("terminal.size", size)
		w.terminal.SetFontSize(float32(size))
	}), widget.NewButton("关闭", func() { w.close(); u.tabs.Remove(w.tab) }))
	terminalPane := container.NewBorder(container.NewBorder(nil, nil, toolbar, nil, find), w.commandBar(), nil, nil, holder)
	w.transfers = container.NewVBox()
	bottom := container.NewAppTabs(container.NewTabItem("文件", w.filePane()), container.NewTabItem("监控", w.monitorPane()), container.NewTabItem("传输", container.NewVScroll(w.transfers)), container.NewTabItem("网络诊断", w.networkPane()))
	split := container.NewVSplit(terminalPane, bottom)
	split.Offset = .64
	w.tab = container.NewTabItem(h.Name, split)
	w.refreshFiles()
	return w
}
func (w *workspace) close() {
	w.once.Do(func() {
		w.cancel()
		for _, s := range w.sessions {
			s.Close()
		}
		for _, v := range w.terminals {
			v.Close()
		}
		for _, r := range w.routers {
			r.Close()
		}
	})
}
func (w *workspace) filePane() fyne.CanvasObject {
	w.dir = widget.NewEntry()
	w.dir.SetText("/")
	w.dir.OnSubmitted = func(string) { w.refreshFiles() }
	w.fileList = widget.NewList(func() int { return len(w.files) }, func() fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewIcon(theme.FileIcon()), widget.NewLabel("大小"), widget.NewLabel("文件名"))
	}, func(i widget.ListItemID, obj fyne.CanvasObject) {
		f := w.files[i]
		box := obj.(*fyne.Container)
		box.Objects[0].(*widget.Label).SetText(f.Name)
		box.Objects[2].(*widget.Label).SetText(fmt.Sprintf("%d B", f.Size))
		icon := box.Objects[1].(*widget.Icon)
		if f.IsDir {
			icon.SetResource(theme.FolderIcon())
		} else {
			icon.SetResource(theme.FileIcon())
		}

	})
	w.fileList.OnSelected = func(i widget.ListItemID) { w.selectedFile = i }
	actions := container.NewHBox(widget.NewButton("打开", w.openSelected), widget.NewButton("上传", w.upload), widget.NewButton("下载", w.download), widget.NewButton("新建目录", w.mkdir), widget.NewButton("重命名", w.rename), widget.NewButton("权限", w.permissions), widget.NewButton("压缩", w.compress), widget.NewButton("删除", w.remove), widget.NewButton("终端进入目录", func() { w.terminal.Send("cd -- " + remote.Quote(w.dir.Text) + "\r") }))
	return container.NewBorder(container.NewVBox(container.NewBorder(nil, nil, widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { w.dir.SetText(path.Dir(w.dir.Text)); w.refreshFiles() }), widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), w.refreshFiles), w.dir), container.NewHScroll(actions)), nil, nil, nil, w.fileList)
}
func (w *workspace) refreshFiles() {
	dir := w.dir.Text
	w.u.work("读取目录", func() error {
		files, e := w.u.Manager.List(w.ctx, w.host.ID, dir)
		if e != nil {
			return e
		}
		fyne.Do(func() {
			if w.dir.Text != dir || w.ctx.Err() != nil {
				return
			}
			w.files = files
			w.selectedFile = -1
			w.fileList.UnselectAll()
			w.fileList.Refresh()
		})
		return nil
	})
}
func (w *workspace) selection() (remote.FileEntry, string, bool) {
	if w.selectedFile < 0 || w.selectedFile >= len(w.files) {
		w.u.error(fmt.Errorf("请选择文件"))
		return remote.FileEntry{}, "", false
	}
	f := w.files[w.selectedFile]
	return f, path.Join(w.dir.Text, f.Name), true
}
func (w *workspace) openSelected() {
	f, p, ok := w.selection()
	if !ok {
		return
	}
	if f.IsDir {
		w.dir.SetText(p)
		w.refreshFiles()
		return
	}
	w.u.work("读取文件", func() error {
		b, hash, e := w.u.Manager.ReadFile(w.ctx, w.host.ID, p)
		if e != nil {
			return e
		}
		fyne.Do(func() {
			editor := widget.NewMultiLineEntry()
			editor.SetText(string(b))
			editor.SetMinRowsVisible(20)
			editor.TextStyle = fyne.TextStyle{Monospace: true}
			d := dialog.NewCustomConfirm(p, "保存", "关闭", editor, func(ok bool) {
				if !ok {
					return
				}
				text := editor.Text
				w.u.work("保存文件", func() error {
					backup, e := w.u.Manager.WriteFile(w.ctx, w.host.ID, p, hash, []byte(text))
					if e == nil {
						_, e = w.u.Store.Event("manual:"+w.host.ID, "file_write", p+" 备份 "+backup)
					}
					return e
				})
			}, w.u.Window)
			d.Resize(fyne.NewSize(900, 650))
			d.Show()
		})
		return nil
	})
}
func (w *workspace) transfer(local, remotePath string, upload, resume bool) {
	ctx, cancel := context.WithCancel(w.ctx)
	label := widget.NewLabel(path.Base(remotePath))
	progress := widget.NewLabel("等待传输")
	row := container.NewBorder(nil, nil, label, widget.NewButton("取消", cancel), progress)
	w.transfers.Add(row)
	go func() {
		defer cancel()
		var last time.Time
		err := w.u.Manager.Transfer(ctx, w.host.ID, local, remotePath, upload, resume, func(n int64) {
			if time.Since(last) < 100*time.Millisecond {
				return
			}
			last = time.Now()
			fyne.Do(func() { progress.SetText(fmt.Sprintf("%d 字节", n)) })
		})
		fyne.Do(func() {
			if err != nil {
				progress.SetText("失败：" + err.Error())
			} else {
				progress.SetText("已完成")
				w.refreshFiles()
			}
		})
	}()
}
func (w *workspace) upload() {
	dialog.ShowFileOpen(func(r fyne.URIReadCloser, e error) {
		if e != nil {
			w.u.error(e)
			return
		}
		if r == nil {
			return
		}
		local := r.URI().Path()
		r.Close()
		dest := path.Join(w.dir.Text, path.Base(local))
		var d dialog.Dialog
		d = dialog.NewCustom("上传文件", "取消", container.NewVBox(widget.NewLabel(dest), container.NewHBox(widget.NewButton("校验并续传", func() { d.Hide(); w.transfer(local, dest, true, true) }), widget.NewButton("覆盖上传", func() { d.Hide(); w.transfer(local, dest, true, false) }))), w.u.Window)
		d.Show()
	}, w.u.Window)
}
func (w *workspace) download() {
	f, p, ok := w.selection()
	if !ok {
		return
	}
	if f.IsDir {
		w.u.error(fmt.Errorf("请先压缩目录后下载"))
		return
	}
	d := dialog.NewFileSave(func(out fyne.URIWriteCloser, e error) {
		if e != nil {
			w.u.error(e)
			return
		}
		if out == nil {
			return
		}
		local := out.URI().Path()
		out.Close()
		w.transfer(local, p, false, false)
	}, w.u.Window)
	d.SetFileName(f.Name)
	d.Show()
}
func (w *workspace) askName(title, initial string, action func(string) error) {
	e := widget.NewEntry()
	e.SetText(initial)
	dialog.ShowForm(title, "确认", "取消", []*widget.FormItem{widget.NewFormItem("名称", e)}, func(ok bool) {
		if !ok {
			return
		}
		name := e.Text
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
			w.u.error(fmt.Errorf("无效名称"))
			return
		}
		w.u.work(title, func() error {
			err := action(name)
			if err == nil {
				fyne.Do(w.refreshFiles)
			}
			return err
		})
	}, w.u.Window)
}
func (w *workspace) mkdir() {
	dir := w.dir.Text
	w.askName("新建目录", "", func(name string) error {
		c, e := w.u.Manager.SFTP(w.ctx, w.host.ID)
		if e != nil {
			return e
		}
		defer c.Close()
		return c.Mkdir(path.Join(dir, name))
	})
}
func (w *workspace) rename() {
	f, p, ok := w.selection()
	if !ok {
		return
	}
	w.askName("重命名", f.Name, func(name string) error {
		c, e := w.u.Manager.SFTP(w.ctx, w.host.ID)
		if e != nil {
			return e
		}
		defer c.Close()
		return c.Rename(p, path.Join(path.Dir(p), name))
	})
}
func (w *workspace) permissions() {
	_, p, ok := w.selection()
	if !ok {
		return
	}
	entry := widget.NewEntry()
	entry.SetText("0644")
	dialog.ShowForm("修改权限", "保存", "取消", []*widget.FormItem{widget.NewFormItem("八进制权限", entry)}, func(ok bool) {
		if !ok {
			return
		}
		mode, e := strconv.ParseUint(entry.Text, 8, 12)
		if e != nil {
			w.u.error(e)
			return
		}
		w.u.work("修改权限", func() error {
			c, e := w.u.Manager.SFTP(w.ctx, w.host.ID)
			if e != nil {
				return e
			}
			defer c.Close()
			return c.Chmod(p, os.FileMode(mode))
		})
	}, w.u.Window)
}
func (w *workspace) remove() {
	f, p, ok := w.selection()
	if !ok {
		return
	}
	dialog.ShowConfirm("删除", "确认删除 "+p+"？", func(ok bool) {
		if !ok {
			return
		}
		w.u.work("删除文件", func() error {
			c, e := w.u.Manager.SFTP(w.ctx, w.host.ID)
			if e != nil {
				return e
			}
			defer c.Close()
			if f.IsDir {
				e = c.RemoveDirectory(p)
			} else {
				e = c.Remove(p)
			}
			if e == nil {
				fyne.Do(w.refreshFiles)
			}
			return e
		})
	}, w.u.Window)
}
func (w *workspace) compress() {
	_, p, ok := w.selection()
	if !ok {
		return
	}
	command := "tar -czf " + remote.Quote(p+".tar.gz") + " -C " + remote.Quote(path.Dir(p)) + " -- " + remote.Quote(path.Base(p))
	w.u.work("压缩文件", func() error {
		r, e := w.u.Executor.Execute(w.ctx, domain.Request{TaskID: "manual:" + w.host.ID, CallID: domain.ID(), HostID: w.host.ID, Operation: "shell", Command: command, TimeoutSeconds: 600})
		if e == nil && r.Status != "succeeded" {
			e = fmt.Errorf("%s", r.Error)
		}
		if e == nil {
			fyne.Do(w.refreshFiles)
		}
		return e
	})
}
func (w *workspace) monitorPane() fyne.CanvasObject {
	status := widget.NewLabel("等待采样")
	cpu := widget.NewProgressBar()
	memory := widget.NewProgressBar()
	cpuText := widget.NewLabel("处理器")
	memoryText := widget.NewLabel("内存")
	text := newReadOnly()
	text.TextStyle = fyne.TextStyle{Monospace: true}
	panels := container.NewBorder(container.NewVBox(status, container.NewGridWithColumns(2, container.NewVBox(cpuText, cpu), container.NewVBox(memoryText, memory))), nil, nil, nil, text)
	go func() {
		var prev *remote.Snapshot
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			snap, e := w.u.Manager.Monitor(w.ctx, w.host.ID, prev)
			if e == nil {
				prev = &snap
			}
			fyne.Do(func() {
				if e != nil {
					status.SetText("采集失败：" + e.Error())
					return
				}
				status.SetText("更新于 " + snap.At.Format("15:04:05"))
				if value, ok := snap.CPU.Value.(float64); ok {
					cpu.SetValue(value / 100)
					cpuText.SetText(fmt.Sprintf("处理器 %.1f%%", value))
				} else {
					cpuText.SetText("处理器：等待下一次采样")
				}
				if mem, ok := snap.Memory.Value.(map[string]uint64); ok {
					memory.SetValue(float64(mem["used"]) / float64(mem["total"]))
					memoryText.SetText(fmt.Sprintf("内存 %.1f / %.1f GiB", float64(mem["used"])/(1<<30), float64(mem["total"])/(1<<30)))
				}
				format := func(m remote.Metric) string {
					if m.State != "ok" {
						return "采集不可用：" + m.Error
					}
					if v, ok := m.Value.(string); ok {
						return v
					}
					b, _ := json.MarshalIndent(m.Value, "", "  ")
					return string(b)
				}
				text.SetText("系统\n" + format(snap.System) + "\n\n负载\n" + format(snap.Load) + "\n\n磁盘\n" + format(snap.Disks) + "\n\n网卡（每秒字节）\n" + format(snap.Network) + "\n\n进程\n" + format(snap.Processes) + "\n\n端口连接\n" + format(snap.Ports))
			})
			select {
			case <-ticker.C:
			case <-w.ctx.Done():
				return
			}
		}
	}()
	return panels
}
func (w *workspace) networkPane() fyne.CanvasObject {
	target := widget.NewEntry()
	target.SetPlaceHolder("目标域名或 IP")
	kind := widget.NewSelect([]string{"Ping", "路由追踪"}, nil)
	kind.SetSelected("Ping")
	output := newReadOnly()
	run := widget.NewButton("诊断", func() {
		destination := target.Text
		if strings.HasPrefix(destination, "-") || destination == "" {
			return
		}
		cmd := "ping -c 4 -- " + remote.Quote(destination)
		if kind.Selected == "路由追踪" {
			cmd = "traceroute -- " + remote.Quote(destination)
		}
		w.u.work("网络诊断", func() error {
			r, e := w.u.Executor.Execute(w.ctx, domain.Request{TaskID: "manual:" + w.host.ID, CallID: domain.ID(), HostID: w.host.ID, Operation: "shell", Command: cmd, TimeoutSeconds: 60})
			fyne.Do(func() { output.SetText(r.Output + "\n" + r.Error) })
			return e
		})
	})
	return container.NewBorder(container.NewBorder(nil, nil, kind, run, target), nil, nil, nil, output)
}

var _ io.Reader
