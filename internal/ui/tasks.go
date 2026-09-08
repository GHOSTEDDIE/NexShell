package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"strings"
	"time"
)

func (u *App) agentPanel() fyne.CanvasObject {
	u.taskStatus = widget.NewLabel("尚未选择任务")
	u.taskText = newReadOnly()
	u.taskText.Wrapping = fyne.TextWrapWord
	u.taskText.SetPlaceHolder("在这里查看计划、操作与验证结果")
	u.taskList = widget.NewSelect(nil, func(label string) {
		tasks, _ := store.All[domain.Task](u.Store, "tasks")
		for _, t := range tasks {
			if strings.HasPrefix(label, t.ID[:8]+" · ") {
				u.taskID = t.ID
				break
			}
		}
		u.updateTask()
	})
	u.taskList.PlaceHolder = "选择任务"
	input := widget.NewMultiLineEntry()
	input.SetPlaceHolder("补充运维目标或约束…")
	input.SetMinRowsVisible(3)
	return container.NewBorder(container.NewVBox(widget.NewLabelWithStyle("运维助手", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), u.taskList, u.taskStatus, widget.NewButton("新建运维任务", u.newTaskDialog)), container.NewVBox(input, container.NewGridWithColumns(2, widget.NewButton("发送", func() {
		id, text := u.taskID, input.Text
		input.SetText("")
		u.work("发送指令", func() error { return u.Agent.Submit(id, text) })
	}), widget.NewButton("停止", func() { u.Agent.Cancel(u.taskID) })), container.NewGridWithColumns(2, widget.NewButton("待确认操作", u.approvalDialog), widget.NewButton("恢复任务", func() { id := u.taskID; u.work("恢复任务", func() error { return u.Agent.Resume(id) }) }))), nil, nil, u.taskText)
}
func (u *App) newTaskDialog() {
	names, ids := u.hostChoices()
	hosts := widget.NewCheckGroup(names, nil)
	for _, name := range names {
		if ids[name] == u.selected {
			hosts.SetSelected([]string{name})
		}
	}
	goal := widget.NewMultiLineEntry()
	goal.SetPlaceHolder("例如：检查 Web 服务异常，定位原因并修复后验证。")
	goal.SetMinRowsVisible(4)
	ops := widget.NewCheckGroup([]string{"读取文件", "修改文件", "重启服务", "安装软件包"}, nil)
	resources := widget.NewMultiLineEntry()
	resources.SetPlaceHolder("每行一个允许操作的文件绝对路径、服务名或软件包名")
	resources.SetMinRowsVisible(3)
	profiles, _ := store.All[domain.ModelProfile](u.Store, "models")
	pnames := []string{}
	pids := map[string]string{}
	for _, p := range profiles {
		label := p.Name + " · " + p.Model
		pnames = append(pnames, label)
		pids[label] = p.ID
	}
	model := widget.NewSelect(pnames, nil)
	if len(pnames) > 0 {
		model.SetSelected(pnames[0])
	}
	d := dialog.NewForm("创建任务并授权", "授权并开始", "取消", []*widget.FormItem{widget.NewFormItem("目标服务器", hosts), widget.NewFormItem("任务目标", goal), widget.NewFormItem("模型", model), widget.NewFormItem("自动执行范围", ops), widget.NewFormItem("允许操作的资源", resources)}, func(ok bool) {
		if !ok {
			return
		}
		var selected []string
		for _, label := range hosts.Selected {
			selected = append(selected, ids[label])
		}
		allowed := []string{"observe", "service_status", "logs"}
		mapping := map[string]string{"读取文件": "file_read", "修改文件": "file_write", "重启服务": "service_restart", "安装软件包": "package_install"}
		for _, label := range ops.Selected {
			allowed = append(allowed, mapping[label])
		}
		g, p, r := goal.Text, pids[model.Selected], splitLines(resources.Text)
		u.work("启动运维任务", func() error {
			t, e := u.Agent.NewTask(g, p, selected, allowed, r)
			if e != nil {
				return e
			}
			fyne.Do(func() { u.taskID = t.ID; u.refreshTasks() })
			return u.Agent.Submit(t.ID, g)
		})
	}, u.Window)
	d.Resize(fyne.NewSize(680, 650))
	d.Show()
}
func (u *App) refreshTasks() {
	tasks, e := store.All[domain.Task](u.Store, "tasks")
	if e != nil {
		return
	}
	var labels []string
	selected := ""
	for _, t := range tasks {
		goal := []rune(t.Goal)
		if len(goal) > 18 {
			goal = goal[:18]
		}
		label := t.ID[:8] + " · " + string(goal)
		labels = append(labels, label)
		if t.ID == u.taskID {
			selected = label
		}
	}
	u.taskList.Options = labels
	u.taskList.Refresh()
	if selected != "" {
		u.taskList.SetSelected(selected)
	}
}
func (u *App) updateTask() {
	if u.taskID == "" {
		return
	}
	var t domain.Task
	if u.Store.Load("tasks", u.taskID, &t) != nil {
		return
	}
	states := map[string]string{"ready": "就绪", "running": "运行中", "awaiting_approval": "等待确认", "interrupted": "已暂停", "failed": "执行失败", "completed": "验证完成", "needs_verification": "等待验证"}
	u.taskStatus.SetText(states[t.Status])
	events, e := u.Store.Events(u.taskID, 0)
	if e != nil {
		return
	}
	var b strings.Builder
	for _, event := range events {
		switch event.Kind {
		case "assistant_delta":
			b.WriteString(event.Text)
		case "user":
			fmt.Fprintf(&b, "\n\n你：%s\n\n", event.Text)
		case "assistant":
			b.WriteString(event.Text)
		case "execution_start":
			fmt.Fprintf(&b, "\n\n执行：%s\n", event.Text)
		case "execution_result":
			fmt.Fprintf(&b, "\n%s\n", event.Text)
		case "execution_delta":
			// The result event includes the completed preview; avoid duplicating it.
			if t.Status == "running" {
				b.WriteString(event.Text)
			}
		case "verified":
			fmt.Fprintf(&b, "\n验证：%s\n", event.Text)
		case "approval_required":
			b.WriteString("\n需要确认具体操作，请点击“待确认操作”。\n")
		}
	}
	if t.Status == "failed" {
		fmt.Fprintf(&b, "\n%s", t.Summary)
	}
	text := b.String()
	if u.taskText.Text != text {
		u.taskText.SetText(text)
	}
}
func (u *App) watchTasks() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fyne.Do(func() {
				select {
				case <-u.ctx.Done():
					return
				default:
				}
				u.updateTask()
			})
		case <-u.ctx.Done():
			return
		}
	}
}
func (u *App) approvalDialog() {
	all, e := store.All[agent.Approval](u.Store, "approvals")
	if e != nil {
		u.error(e)
		return
	}
	for _, a := range all {
		if a.Request.TaskID != u.taskID || a.Decided {
			continue
		}
		id := u.taskID
		approval := a
		h, _ := u.Store.Host(a.Request.HostID)
		body := fmt.Sprintf("服务器：%s (%s@%s)\n操作：%s\n资源：%s\n\n命令：\n%s\n\n拟写入内容：\n%s", h.Name, h.User, h.Address, a.Request.Operation, a.Request.Resource, a.Request.Command, a.Request.Content)
		preview := newReadOnly()
		preview.SetText(body)
		preview.SetMinRowsVisible(16)
		d := dialog.NewCustomConfirm("确认本次操作", "允许本次", "拒绝", preview, func(ok bool) { u.work("处理确认", func() error { return u.Agent.Decide(id, approval.Digest, ok) }) }, u.Window)
		d.Resize(fyne.NewSize(760, 600))
		d.Show()
		return
	}
	dialog.ShowInformation("待确认操作", "当前没有待确认操作。", u.Window)
}
