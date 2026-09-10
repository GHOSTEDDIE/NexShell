package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"strings"
)

func (u *App) memoryDialog() {
	names := []string{"通用偏好"}
	scopes := map[string]string{"通用偏好": "preferences"}
	for _, h := range u.hosts {
		name := h.Name + " · " + shortID(h.ID)
		names = append(names, name)
		scopes[name] = "host:" + h.ID
	}
	scope := widget.NewSelect(names, nil)
	files := widget.NewSelect(nil, nil)
	text := newReadOnly()
	text.SetMinRowsVisible(12)
	refresh := func() {
		entries, err := u.Agent.ListMemory(scopes[scope.Selected])
		if err != nil {
			u.error(err)
			return
		}
		files.Options = nil
		for _, entry := range entries {
			files.Options = append(files.Options, entry.Path)
		}
		files.ClearSelected()
		files.Refresh()
		text.SetText("")
	}
	scope.OnChanged = func(string) { refresh() }
	scope.SetSelected(names[0])
	files.OnChanged = func(name string) {
		entries, err := u.Agent.ListMemory(scopes[scope.Selected])
		if err != nil {
			u.error(err)
			return
		}
		for _, entry := range entries {
			if entry.Path == name {
				text.SetText(entry.Content)
				return
			}
		}
		text.SetText("")
	}
	enabled := widget.NewCheck("自动记忆", nil)
	on, err := u.Agent.MemoryEnabled()
	if err != nil {
		u.error(err)
		return
	}
	enabled.SetChecked(on)
	enabled.OnChanged = func(v bool) { u.error(u.Agent.SetMemoryEnabled(v)) }
	remove := widget.NewButton("删除所选记忆", func() {
		if files.Selected == "" {
			return
		}
		if err := u.Agent.DeleteMemory(scopes[scope.Selected], files.Selected); err != nil {
			u.error(err)
			return
		}
		refresh()
	})
	content := container.NewBorder(container.NewVBox(enabled, scope, files), remove, nil, nil, text)
	d := dialog.NewCustom("管理记忆", "关闭", content, u.Window)
	d.Resize(fyne.NewSize(660, 540))
	d.Show()
}
func (u *App) resumeTask() {
	id := u.taskID
	var task domain.Task
	if err := u.Store.Load("tasks", id, &task); err != nil {
		u.error(err)
		return
	}
	lines := []string{"确认当前服务器与原操作范围后继续："}
	for _, host := range task.Grant.HostIDs {
		h, err := u.Store.Host(host)
		if err != nil {
			u.error(err)
			return
		}
		lines = append(lines, fmt.Sprintf("%s · %s@%s", h.Name, h.User, h.Address))
	}
	lines = append(lines, "操作："+strings.Join(task.Grant.Operations, ", "), "资源："+strings.Join(task.Grant.Resources, ", "))
	dialog.ShowConfirm("恢复任务", strings.Join(lines, "\n"), func(ok bool) {
		if ok {
			u.work("恢复任务", func() error { return u.Agent.Resume(id) })
		}
	}, u.Window)
}
