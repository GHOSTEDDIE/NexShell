package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"strings"
)

func (u *App) agentPanel() fyne.CanvasObject {
	u.taskStatus = widget.NewLabel("")
	u.taskStatus.SizeName = sizeMeta
	u.taskStatus.Importance = widget.LowImportance
	u.taskStatus.Hide()
	u.taskText = newReadOnly()
	u.conversationView = newConversationView()
	u.taskList = widget.NewSelect(nil, func(label string) {
		tasks, _ := store.All[domain.Task](u.Store, "tasks")
		for _, t := range tasks {
			if strings.HasPrefix(label, shortID(t.ID)+" · ") {
				u.taskID = t.ID
				u.updateTask()
				break
			}
		}
	})
	u.taskList.PlaceHolder = "选择任务"
	input := newChatInput()
	input.SetPlaceHolder("输入消息、本机文件路径，或粘贴文件…")
	u.composer = u.newAttachmentComposer(input)
	composer := u.composer
	send := func() {
		id, text, files := u.taskID, input.Text, composer.paths()
		version := composer.revision
		if strings.TrimSpace(text) == "" && len(files) == 0 {
			return
		}
		if id == "" {
			u.newConversationWithFiles(text, files, func() {
				if composer.revision == version {
					composer.clear()
				}
			})
			return
		}
		text = u.conversationInput(id, text)
		u.work("发送指令", func() error {
			if err := u.Agent.SubmitFiles(id, text, files); err != nil {
				return err
			}
			fyne.Do(func() {
				if composer.revision == version {
					composer.clear()
				}
			})
			return nil
		})
	}
	input.Submit = send
	u.modelSelect = widget.NewSelect(nil, nil)
	u.modelSelect.PlaceHolder = "默认模型"
	u.refreshModelChoices()
	u.modelSelect.OnChanged = func(label string) {
		profiles, _ := store.All[domain.ModelProfile](u.Store, "models")
		for _, p := range profiles {
			if p.Name+" · "+p.Model == label {
				u.preferredModel = p.ID
				if u.taskID != "" {
					var t domain.Task
					if u.Store.Load("tasks", u.taskID, &t) == nil && t.ProfileID != p.ID {
						u.taskID = ""
						u.updateTask()
						u.conversationView.reset("")
					}
				}
				break
			}
		}
	}
	var menuButton *actionButton
	menuButton = action("", designIcon("more"), func() {
		history := func() { showMotionDialog("历史对话", "关闭", sized(u.taskList, 480, 36), u.Window) }
		menu := fyne.NewMenu("", fyne.NewMenuItem("新对话", func() { u.newConversationDialog("") }), fyne.NewMenuItem("历史对话", history), fyne.NewMenuItem("运维任务", u.newTaskDialog), fyne.NewMenuItem("待确认操作", u.approvalDialog), fyne.NewMenuItem("恢复任务", u.resumeTask), fyne.NewMenuItem("管理记忆", u.memoryDialog))
		widget.NewPopUpMenu(menu, u.Window.Canvas()).ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(menuButton).Add(fyne.NewPos(0, 30)))
	})
	header := sized(inset(container.NewBorder(nil, nil, container.NewHBox(widget.NewIcon(designIcon("spark")), headingText("助手")), container.NewHBox(menuButton, action("", designIcon("close"), u.toggleAssistant))), 0, 14, 0, 18), 0, 38)
	u.assistantContext = metaText("尚未连接服务器")
	context := inset(container.NewBorder(nil, nil, inset(widget.NewIcon(designIcon("server")), 0, 8, 0, 0), nil, u.assistantContext), 12, 18, 12, 18)
	sendButton := action("", designIcon("send"), send)
	sendButton.primary = true
	u.stopAction = action("停止", nil, func() { u.Agent.Cancel(u.taskID) })
	u.stopAction.Hide()
	u.approvalAction = action("待确认操作", nil, u.approvalDialog)
	u.approvalAction.Hide()
	tools := container.NewBorder(nil, nil, nil, container.NewHBox(action("添加文件", theme.DocumentIcon(), u.chooseAttachment), action("", designIcon("settings"), u.modelDialog), sendButton), u.modelSelect)
	compose := panel(padded(container.NewVBox(composer.rows, container.NewThemeOverride(input, componentTheme{body: true, clearInput: true}), tools), 10), theme.ColorNameBackground, true, 7)
	composer.area = compose
	bottom := inset(container.NewVBox(container.NewHBox(u.taskStatus, u.stopAction, u.approvalAction), compose, metaText("Enter 发送 · Shift + Enter 换行")), 4, 16, 12, 16)
	return edge(edge(header, nil, nil, nil, context), bottom, nil, nil, u.conversationView.scroll)
}
func (u *App) refreshModelChoices() {
	if u.modelSelect == nil {
		return
	}
	profiles, e := store.All[domain.ModelProfile](u.Store, "models")
	if e != nil {
		u.error(e)
		return
	}
	options := []string{}
	for _, p := range profiles {
		options = append(options, p.Name+" · "+p.Model)
	}
	callback := u.modelSelect.OnChanged
	u.modelSelect.OnChanged = nil
	u.modelSelect.Options = options
	if len(options) > 0 {
		index := 0
		for i, p := range profiles {
			if p.ID == u.preferredModel {
				index = i
				break
			}
		}
		u.modelSelect.SetSelected(options[index])
		u.preferredModel = profiles[index].ID
	} else {
		u.modelSelect.ClearSelected()
		u.preferredModel = ""
	}
	u.modelSelect.Refresh()
	u.modelSelect.OnChanged = callback
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
	roots := widget.NewMultiLineEntry()
	roots.SetPlaceHolder("可选，每行一个本机目录的绝对路径")
	roots.SetMinRowsVisible(2)
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
	d := newMotionForm("创建任务并授权", "授权并开始", "取消", []*widget.FormItem{widget.NewFormItem("目标服务器", hosts), widget.NewFormItem("任务目标", goal), widget.NewFormItem("模型", model), widget.NewFormItem("自动执行范围", ops), widget.NewFormItem("允许操作的资源", resources), widget.NewFormItem("可读取的本机目录", roots)}, func(ok bool) {
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
		g, p, r, local := goal.Text, pids[model.Selected], splitLines(resources.Text), splitLines(roots.Text)
		u.work("启动运维任务", func() error {
			t, e := u.Agent.NewTask(g, p, selected, allowed, r, local...)
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
		label := shortID(t.ID) + " · " + string(goal)
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
func (u *App) approvalDialog() {
	all, e := store.All[agent.Approval](u.Store, "approvals")
	if e != nil {
		u.error(e)
		return
	}
	for _, a := range all {
		var task domain.Task
		if u.Store.Load("tasks", u.taskID, &task) != nil {
			return
		}
		if a.Request.TaskID != u.taskID || a.Decided || a.GrantID != task.Grant.ID {
			continue
		}
		id := u.taskID
		approval := a
		h, _ := u.Store.Host(a.Request.HostID)
		body := fmt.Sprintf("服务器：%s (%s@%s)\n操作：%s\n资源：%s\n\n命令：\n%s\n\n拟写入内容：\n%s", h.Name, h.User, h.Address, a.Request.Operation, a.Request.Resource, a.Request.Command, a.Request.Content)
		if a.Request.HostID == "" {
			body = fmt.Sprintf("本机操作：%s", a.Request.Operation)
		}
		if a.Request.LocalPath != "" {
			body += "\n本机路径：" + a.Request.LocalPath
		}
		if a.Request.Operation == "local_read" {
			body += fmt.Sprintf("\n读取起点：%d 字节", a.Request.ReadOffset)
		}
		if a.Request.Operation == "upload_local" {
			mode := map[string]string{"": "仅新建", "new": "仅新建", "replace": "覆盖", "resume": "续传"}[a.Request.UploadMode]
			body += "\n上传方式：" + mode + "\n文件 SHA-256：" + a.Request.SourceHash
		}
		body = "发起助手：" + agentLabel(a.Request.AgentName) + "\n子运行：" + a.Request.RunID + "\n" + body
		preview := newReadOnly()
		preview.SetText(body)
		preview.SetMinRowsVisible(16)
		d := newMotionConfirm("确认本次操作", "允许本次", "拒绝", preview, func(ok bool) { u.work("处理确认", func() error { return u.Agent.Decide(id, approval.Digest, ok) }) }, u.Window)
		d.Resize(fyne.NewSize(760, 600))
		d.Show()
		return
	}
	showMotionInformation("待确认操作", "当前没有待确认操作。", u.Window)
}
