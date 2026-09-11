package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/nativefiles"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type workspace struct {
	bottomHeight    float32
	monitor         fyne.CanvasObject
	listedDirectory string
	activeSession   atomic.Pointer[remote.TerminalSession]
	fileArea        fyne.CanvasObject
	bottomTabs      *tabView
	fileTab         *container.TabItem
	followDirectory *widget.Check
	directoryStatus *widget.Label
	terminalIDs     []string
	u               *App
	host            domain.Host
	session         *remote.TerminalSession
	terminal        *terminal.View
	terminals       []*terminal.View
	sessions        []*remote.TerminalSession
	tab             *container.TabItem
	ctx             context.Context
	cancel          context.CancelFunc
	once            sync.Once
	files           []remote.FileEntry
	fileList        *widget.List
	dir             *widget.Entry
	selectedFile    int
	transfers       *fyne.Container
	routers         []*transfer.Router
}

func (u *App) newWorkspace(h domain.Host, s *remote.TerminalSession) *workspace {
	ctx, cancel := context.WithCancel(u.ctx)
	w := &workspace{u: u, host: h, session: s, ctx: ctx, cancel: cancel, selectedFile: -1}
	w.terminal = w.newTerminal(s)
	w.terminal.OnResize = func(rows, cols int) { _ = s.Resize(rows, cols) }
	w.terminals = []*terminal.View{w.terminal}
	w.sessions = []*remote.TerminalSession{s}
	w.activeSession.Store(s)
	w.monitor = w.monitorPane()
	w.layoutWorkspace()
	go w.watchDirectory()
	return w
}

func (w *workspace) layoutWorkspace() {
	u, h, ctx := w.u, w.host, w.ctx
	holder := container.NewStack(w.terminal)
	splitButton := action("", designIcon("split"), func() {
		u.work("创建分屏", func() error {
			next, e := u.Manager.Terminal(ctx, h.ID, 24, 80)
			if e != nil {
				return e
			}
			fyne.Do(func() {
				if ctx.Err() != nil {
					go next.Close()
					return
				}
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
	})
	var more *actionButton
	more = action("", designIcon("more"), func() {
		search := func() {
			find := widget.NewEntry()
			find.SetPlaceHolder("搜索终端输出")
			find.OnSubmitted = func(q string) { count := w.terminal.Search(q); u.status.SetText(fmt.Sprintf("找到 %d 行", count)) }
			showMotionDialog("搜索终端", "关闭", sized(find, 440, 36), u.Window)
		}
		menu := fyne.NewMenu("", fyne.NewMenuItem("搜索终端输出", search), fyne.NewMenuItem("快捷命令", u.snippetsDialog), fyne.NewMenuItem("命令输入栏", func() { showMotionDialog("执行命令", "关闭", sized(w.commandBar(), 650, 40), u.Window) }), fyne.NewMenuItem("终端字号", func() { u.settingsDialog("appearance") }), fyne.NewMenuItem("关闭会话", func() { u.closeWorkspace(w.tab) }))
		widget.NewPopUpMenu(menu, u.Window.Canvas()).ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(more).Add(fyne.NewPos(0, 30)))
	})
	badge := panel(padded(textUI("已连接", sizeMeta, theme.ColorNameSuccess, false), 4), colorSuccessBG, false, 4)
	toolbar := sized(inset(container.NewBorder(nil, nil, container.NewHBox(textUI(h.User+" @ "+h.Address, sizeControl, colorMuted, false), badge), container.NewHBox(action("表格查看", nil, w.showOutputTable), splitButton, more)), 6, 18, 6, 18), 0, 44)
	terminalPane := edge(toolbar, nil, nil, nil, panel(inset(holder, 20, 22, 20, 22), "terminalBackground", false, 0))
	w.transfers = container.NewVBox()
	w.fileArea = w.filePane()
	w.fileTab = container.NewTabItem("文件", w.fileArea)
	w.bottomTabs = newTabView(false, w.fileTab, container.NewTabItem("传输", container.NewVScroll(w.transfers)), container.NewTabItem("网络诊断", w.networkPane()))
	w.bottomHeight = savedPanelSize(u.UI.Preferences(), "layout.bottomHeight", 250)
	w.tab = container.NewTabItemWithIcon(h.Name, fyne.NewStaticResource("connected.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18"><circle cx="9" cy="9" r="3" fill="#51ac83"/></svg>`)), container.New(terminalFileLayout{w: w}, terminalPane, w.bottomTabs, newResizeDivider(true, func(delta float32) {
		content := w.tab.Content.(*fyne.Container)
		w.bottomHeight = boundedPanelSize(w.bottomTabs.Size().Height-delta, minimumToolsHeight, content.Size().Height-minimumTerminalHeight-dividerSize)
		content.Refresh()
	}, func() { u.UI.Preferences().SetFloat("layout.bottomHeight", float64(w.bottomHeight)) })))
}

type terminalFileLayout struct{ w *workspace }

func (terminalFileLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(480, 500) }
func (l terminalFileLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	available := max(0, s.Height-dividerSize)
	bottom := boundedPanelSize(l.w.bottomHeight, minimumToolsHeight, available-minimumTerminalHeight)
	top := max(0, available-bottom)
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(fyne.NewSize(s.Width, top))
	o[2].Move(fyne.NewPos(0, top))
	o[2].Resize(fyne.NewSize(s.Width, dividerSize))
	o[1].Move(fyne.NewPos(0, top+dividerSize))
	o[1].Resize(fyne.NewSize(s.Width, bottom))
}

// closeWorkspace runs on the UI thread. Drop ownership before waiting on any
// network cleanup, so a slow peer cannot block closing or retain dead tabs.
func (u *App) closeWorkspace(item *container.TabItem) {
	if item == u.homeTab {
		return
	}
	for i, w := range u.workspaces {
		if w.tab != item {
			continue
		}
		u.workspaces = slices.Delete(u.workspaces, i, i+1)
		w.cancel()
		for _, id := range w.terminalIDs {
			u.terminalsDesktop.Remove(id)
		}
		go w.close()
		break
	}
	u.tabs.Remove(item)
	if len(u.tabs.Items) == 0 {
		u.tabs.Append(u.homeTab)
		u.tabs.Select(u.homeTab)
	}
	// Fyne 2.8 Remove shortens Items without clearing the removed tail slot.
	// Clear that capacity too, otherwise its backing array retains tab content.
	clear(u.tabs.Items[len(u.tabs.Items):cap(u.tabs.Items)])
	u.selectWorkspace(u.tabs.Selected())
}

func (w *workspace) close() {
	w.once.Do(func() {
		w.cancel()
		for _, id := range w.terminalIDs {
			w.u.terminalsDesktop.Remove(id)
		}
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
	w.dir.SetPlaceHolder("正在读取终端目录…")
	w.dir.OnSubmitted = func(string) { w.refreshFiles() }
	w.directoryStatus = widget.NewLabel("")
	w.directoryStatus.SizeName = sizeMeta
	w.directoryStatus.Importance = widget.LowImportance
	w.followDirectory = widget.NewCheck("跟随终端目录", nil)
	w.followDirectory.SetChecked(true)
	w.followDirectory.OnChanged = func(enabled bool) {
		if enabled {
			w.dir.SetText("")
		}
	}
	w.fileList = widget.NewList(func() int { return len(w.files) }, func() fyne.CanvasObject { return newFileRow() }, func(i widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fileRow)
		row.setEntry(w.files[i])
		row.open = func() { w.selectedFile = i; w.openSelected() }
		row.selectFile = func() { w.fileList.Select(i) }
		row.menu = func(pos fyne.Position) {
			w.selectedFile = i
			widget.NewPopUpMenu(w.fileMenu(), w.u.Window.Canvas()).ShowAtPosition(pos)
		}
	})
	w.fileList.HideSeparators = true
	w.fileList.OnSelected = func(i widget.ListItemID) { w.selectedFile = i }
	var more *actionButton
	more = action("", designIcon("more"), func() {
		widget.NewPopUpMenu(w.fileMenu(), w.u.Window.Canvas()).ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(more).Add(fyne.NewPos(0, 30)))
	})
	tools := container.NewHBox(action("", designIcon("refresh"), w.refreshFiles), action("", designIcon("upload"), w.upload), action("", designIcon("folder"), w.mkdir), more)
	back := action("", designIcon("arrow-left"), func() { w.dir.SetText(path.Dir(w.dir.Text)); w.refreshFiles() })
	toolbar := sized(inset(container.NewBorder(nil, nil, back, tools, container.NewThemeOverride(w.dir, componentTheme{clearInput: true})), 3, 12, 3, 12), 0, 42)
	header := panel(sized(container.New(fileColumns{}, metaText("名称"), metaText("大小"), metaText("修改时间")), 0, 30), colorSoft, false, 0)
	return edge(edge(toolbar, nil, nil, nil, header), nil, nil, nil, w.fileList)
}
func (w *workspace) fileMenu() *fyne.Menu {
	follow := fyne.NewMenuItem("跟随终端目录", func() { w.followDirectory.SetChecked(!w.followDirectory.Checked) })
	follow.Checked = w.followDirectory.Checked
	return fyne.NewMenu("", fyne.NewMenuItem("打开", w.openSelected), fyne.NewMenuItem("下载", w.download), fyne.NewMenuItem("重命名", w.rename), fyne.NewMenuItem("权限", w.permissions), fyne.NewMenuItem("压缩", w.compress), fyne.NewMenuItem("删除", w.remove), fyne.NewMenuItemSeparator(), follow, fyne.NewMenuItem("终端进入目录", func() { w.terminal.Send("cd -- " + remote.Quote(w.dir.Text) + "\r") }))
}
func (w *workspace) refreshFiles() {
	dir := w.dir.Text
	if dir == "" {
		return
	}
	if w.listedDirectory != dir {
		w.files = nil
		w.selectedFile = -1
		w.fileList.UnselectAll()
		w.fileList.Refresh()
	}
	w.u.work("读取目录", func() error {
		files, e := w.u.Manager.List(w.ctx, w.host.ID, dir)
		if e != nil {
			return e
		}
		fyne.Do(func() {
			if w.dir.Text != dir || w.ctx.Err() != nil {
				return
			}
			w.listedDirectory = dir
			w.files = files
			w.selectedFile = -1
			w.fileList.UnselectAll()
			w.fileList.Refresh()
		})
		return nil
	})
}
func (w *workspace) selection() (remote.FileEntry, string, bool) {
	if w.listedDirectory != w.dir.Text || w.selectedFile < 0 || w.selectedFile >= len(w.files) {
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
			d := newMotionConfirm(p, "保存", "关闭", editor, func(ok bool) {
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
func (w *workspace) transfer(local, remotePath string, upload, resume bool, exclusive ...bool) {
	ctx, cancel := context.WithCancel(w.ctx)
	label := widget.NewLabel(path.Base(remotePath))
	progress := widget.NewLabel("等待传输")
	row := container.NewBorder(nil, nil, label, widget.NewButton("取消", cancel), progress)
	w.transfers.Add(row)
	if upload {
		w.bottomTabs.Select(w.bottomTabs.Items[1])
	}
	go func() {
		defer cancel()
		var last time.Time
		progressFn := func(n int64) {
			if time.Since(last) < 100*time.Millisecond {
				return
			}
			last = time.Now()
			fyne.Do(func() { progress.SetText(fmt.Sprintf("%d 字节", n)) })
		}
		var err error
		if len(exclusive) > 0 && exclusive[0] {
			err = w.u.Manager.UploadNew(ctx, w.host.ID, local, remotePath, progressFn)
		} else {
			err = w.u.Manager.Transfer(ctx, w.host.ID, local, remotePath, upload, resume, progressFn)
		}
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
	directory := w.dir.Text
	w.u.pickFiles(w.ctx, nativefiles.Request{Mode: nativefiles.OpenMultiple, Title: "选择上传文件"}, func(paths []string, err error) {
		if err != nil {
			w.u.error(err)
			return
		}
		if len(paths) > 0 {
			w.queueUploads(paths, directory)
		}
	})
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
	w.u.pickFiles(w.ctx, nativefiles.Request{Mode: nativefiles.Save, Title: "保存下载文件", Filename: f.Name}, func(paths []string, err error) {
		if err != nil {
			w.u.error(err)
			return
		}
		if len(paths) > 0 {
			w.transfer(paths[0], p, false, false)
		}
	})
}
func (w *workspace) askName(title, initial string, action func(string) error) {
	e := widget.NewEntry()
	e.SetText(initial)
	showMotionForm(title, "确认", "取消", []*widget.FormItem{widget.NewFormItem("名称", e)}, func(ok bool) {
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
	showMotionForm("修改权限", "保存", "取消", []*widget.FormItem{widget.NewFormItem("八进制权限", entry)}, func(ok bool) {
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
	showMotionConfirm("删除", "确认删除 "+p+"？", func(ok bool) {
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
