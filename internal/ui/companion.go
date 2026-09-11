package ui

import (
	"context"
	"errors"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
)

func (u *App) openAgentTerminal(ctx context.Context, host string) (string, error) {
	h, err := u.Store.Host(host)
	if err != nil {
		return "", err
	}
	session, err := u.Manager.Terminal(ctx, host, 24, 80)
	if err != nil {
		var key *remote.HostKeyError
		if !errors.As(err, &key) || key.Changed {
			return "", err
		}
		confirmed := make(chan bool, 1)
		fyne.Do(func() {
			if u.closing || ctx.Err() != nil {
				confirmed <- false
				return
			}
			showMotionConfirm("核对服务器身份", key.Address+"\n"+key.Fingerprint+"\n确认指纹后允许助手连接。", func(ok bool) { confirmed <- ok }, u.Window)
		})
		select {
		case ok := <-confirmed:
			if !ok {
				return "", errors.New("连接未获确认")
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if err = ctx.Err(); err != nil {
			return "", err
		}
		if err = u.Manager.Trust(key); err != nil {
			return "", err
		}
		session, err = u.Manager.Terminal(ctx, host, 24, 80)
		if err != nil {
			return "", err
		}
	}
	type opened struct {
		id  string
		err error
	}
	done := make(chan opened, 1)
	fyne.Do(func() {
		if u.closing || ctx.Err() != nil {
			session.Close()
			done <- opened{err: errors.New("连接已取消")}
			return
		}
		ws := u.newWorkspace(h, session)
		u.showWorkspace(ws)
		done <- opened{id: ws.terminalIDs[0]}
	})
	select {
	case out := <-done:
		return out.id, out.err
	case <-ctx.Done():
		session.Close()
		return "", ctx.Err()
	}
}

func (u *App) newConversationDialog(first string, onCreated ...func()) {
	u.newConversationWithFiles(first, nil, onCreated...)
}
func (u *App) newConversationWithFiles(first string, files []string, onCreated ...func()) {
	profiles, err := store.All[domain.ModelProfile](u.Store, "models")
	if err != nil {
		u.error(err)
		return
	}
	if len(profiles) == 0 {
		u.modelDialog()
		return
	}
	names := []string{}
	ids := map[string]string{}
	for _, p := range profiles {
		label := p.Name + " · " + p.Model + " · " + shortID(p.ID)
		names = append(names, label)
		ids[label] = p.ID
	}
	model := widget.NewSelect(names, nil)
	model.SetSelected(names[0])
	for _, name := range names {
		if ids[name] == u.preferredModel {
			model.SetSelected(name)
		}
	}
	hostNames, hostIDs := u.hostChoices()
	hosts := widget.NewCheckGroup(hostNames, nil)
	selected := u.selected
	for _, info := range u.terminalsDesktop.List(allHostIDs(u.hosts)) {
		if info.Active {
			selected = info.HostID
		}
	}
	for _, name := range hostNames {
		if hostIDs[name] == selected {
			hosts.SetSelected([]string{name})
		}
	}
	note := widget.NewLabel("选择可访问的服务器，也可以直接聊天或分析附件。上传与变更操作会单独确认。")
	note.Wrapping = fyne.TextWrapWord
	d := newMotionConfirm("新对话", "开始对话", "取消", container.NewBorder(container.NewVBox(note, widget.NewForm(widget.NewFormItem("模型", model))), nil, nil, nil, container.NewVScroll(hosts)), func(ok bool) {
		if !ok {
			return
		}
		allowed := []string{}
		for _, name := range hosts.Selected {
			allowed = append(allowed, hostIDs[name])
		}
		t, err := u.Agent.NewConversation(ids[model.Selected], allowed)
		if err != nil {
			u.error(err)
			return
		}
		u.taskID = t.ID
		u.refreshTasks()
		u.taskText.SetText("")
		u.updateTask()
		if first != "" || len(files) > 0 {
			message := u.conversationInput(t.ID, first)
			u.work("发送消息", func() error {
				if err := u.Agent.SubmitFiles(t.ID, message, files); err != nil {
					return err
				}
				fyne.Do(func() {
					for _, callback := range onCreated {
						callback()
					}
				})
				return nil
			})
		}
	}, u.Window)
	d.Resize(fyne.NewSize(600, 480))
	d.Show()
}
func allHostIDs(hosts []domain.Host) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.ID)
	}
	return out
}

// Capture the user's selected terminal at send time; later tab switches do not retarget the request.
func (u *App) conversationInput(id, text string) string {
	var task domain.Task
	if u.Store.Load("tasks", id, &task) != nil || task.Mode != "conversation" {
		return text
	}
	for _, info := range u.terminalsDesktop.List(task.Grant.HostIDs) {
		if info.Active {
			return text + "\n\n[发送时的当前终端：host_id=" + info.HostID + " terminal_id=" + info.ID + "]"
		}
	}
	return text + "\n\n[发送时没有当前已授权终端。请明确选择目标后操作。]"
}
