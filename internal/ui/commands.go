package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"sort"
	"time"
)

type commandRecord struct {
	ID, HostID, Command string
	At                  time.Time
}

func (w *workspace) commandBar() fyne.CanvasObject {
	records, _ := store.All[commandRecord](w.u.Store, "commands")
	sort.Slice(records, func(i, j int) bool { return records[i].At.After(records[j].At) })
	var history []string
	seen := map[string]bool{}
	for _, r := range records {
		if r.HostID == w.host.ID && !seen[r.Command] {
			history = append(history, r.Command)
			seen[r.Command] = true
		}
	}
	entry := widget.NewSelectEntry(history)
	entry.SetPlaceHolder("输入命令，按 Enter 执行")
	remember := widget.NewCheck("记住命令", nil)
	entry.OnSubmitted = func(command string) {
		if command == "" {
			return
		}
		if remember.Checked {
			r := commandRecord{domain.ID(), w.host.ID, command, time.Now()}
			if e := w.u.Store.Put("commands", r.ID, r); e != nil {
				w.u.error(e)
				return
			}
			history = append([]string{command}, history...)
			entry.SetOptions(history)
		}
		w.terminal.Send(command + "\r")
		entry.SetText("")
	}
	return container.NewBorder(nil, nil, nil, remember, entry)
}
func (u *App) insertSnippet(template string) {
	send := func(command string) {
		for _, w := range u.workspaces {
			if w.tab == u.tabs.Selected() {
				w.terminal.Send(command)
				return
			}
		}
		u.error(fmt.Errorf("请先打开终端"))
	}
	names := remote.Parameters(template)
	if len(names) == 0 {
		send(template)
		return
	}
	values := map[string]*widget.Entry{}
	var items []*widget.FormItem
	for _, name := range names {
		entry := widget.NewEntry()
		values[name] = entry
		items = append(items, widget.NewFormItem(name, entry))
	}
	showMotionForm("填写命令参数", "插入", "取消", items, func(ok bool) {
		if !ok {
			return
		}
		text := map[string]string{}
		for name, entry := range values {
			text[name] = entry.Text
		}
		command, e := remote.ExpandSnippet(template, text)
		if e != nil {
			u.error(e)
			return
		}
		send(command)
	}, u.Window)
}
func (u *App) deleteHost() {
	h, e := u.Store.Host(u.selected)
	if e != nil {
		u.error(fmt.Errorf("请选择服务器"))
		return
	}
	showMotionConfirm("删除服务器配置", h.Name+" · "+h.Address, func(ok bool) {
		if !ok {
			return
		}
		for _, other := range u.hosts {
			if other.JumpID == h.ID {
				u.error(fmt.Errorf("该服务器仍被 %s 用作跳板机", other.Name))
				return
			}
		}
		for _, w := range u.workspaces {
			if w.host.ID == h.ID {
				w.close()
				u.tabs.Remove(w.tab)
			}
		}
		u.Manager.Disconnect(h.ID)
		if e := u.Store.Delete("hosts", h.ID); e != nil {
			u.error(e)
			return
		}
		_ = (store.Credentials{}).Delete("host:" + h.ID)
		u.selected = ""
		u.refreshHosts()
	}, u.Window)
}
