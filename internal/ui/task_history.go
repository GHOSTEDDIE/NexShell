package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
)

type taskSource interface {
	Load(string, string, any) error
	Events(string, int64) ([]domain.Event, error)
}

// A worker owns the history for just the selected task. Keep both presentations
// so completion can hide execution deltas without rereading earlier events.
type taskHistory struct {
	messages         []chatMessage
	id               string
	after            int64
	running, settled strings.Builder
	status, summary  string
	loaded           bool
}
type taskUpdate struct {
	id, status, text string
	messages         []chatMessage
}

func (h *taskHistory) load(source taskSource, id string) (taskUpdate, bool, error) {
	if h.id != id {
		*h = taskHistory{id: id}
	}
	var task domain.Task
	if err := source.Load("tasks", id, &task); err != nil {
		return taskUpdate{}, false, err
	}
	events, err := source.Events(id, h.after)
	if err != nil {
		return taskUpdate{}, false, err
	}
	changed := !h.loaded || h.status != task.Status || h.summary != task.Summary || len(events) > 0
	for _, event := range events {
		h.messages = appendChatEvent(h.messages, event)
		text := taskEventText(event)
		h.running.WriteString(text)
		if event.Kind != "execution_delta" {
			h.settled.WriteString(text)
		}
		h.after = event.Sequence
	}
	h.loaded = true
	h.status, h.summary = task.Status, task.Summary
	if !changed {
		return taskUpdate{}, false, nil
	}
	text := h.settled.String()
	if task.Status == "running" {
		text = h.running.String()
	}
	if task.Status == "failed" {
		text += "\n" + task.Summary
	}
	messages := append([]chatMessage(nil), h.messages...)
	if task.Status == "blocked" {
		text += "\n" + task.Summary
		messages = append(messages, chatMessage{"notice", task.Summary})
	}
	if task.Status == "failed" {
		messages = append(messages, chatMessage{"execution", task.Summary}, chatMessage{"notice", "本轮排查暂时无法继续，请查看执行记录中的原因后继续提问。"})
	}
	return taskUpdate{id, task.Status, text, messages}, true, nil
}
func taskEventText(event domain.Event) string {
	switch event.Kind {
	case "assistant_delta", "assistant", "execution_delta":
		return event.Text
	case "user":
		return fmt.Sprintf("\n\n你：%s\n\n", event.Text)
	case "execution_start":
		return fmt.Sprintf("\n\n执行：%s\n", event.Text)
	case "execution_result":
		return fmt.Sprintf("\n%s\n", event.Text)
	case "verified":
		return fmt.Sprintf("\n验证：%s\n", event.Text)
	case "approval_required":
		return "\n需要确认具体操作，请点击“待确认操作”。\n"
	}
	return ""
}

type taskSelection struct {
	id       string
	revision uint64
}

// Called only on the UI thread; keep the latest selection rather than queueing
// an unbounded number of database reads during rapid task switches.
func (u *App) updateTask() {
	if u.conversationView != nil && u.conversationView.id != u.taskID {
		u.conversationView.reset(u.taskID)
		u.taskStatus.SetText("")
		u.taskStatus.Hide()
		u.stopAction.Hide()
		u.approvalAction.Hide()
	}
	u.taskRevision++
	selection := taskSelection{u.taskID, u.taskRevision}
	select {
	case <-u.taskRequests:
	default:
	}
	u.taskRequests <- selection
}
func (u *App) watchTasks() {
	watchTaskUpdates(u.ctx, u.Store, u.taskRequests, func(selection taskSelection, update taskUpdate) {
		// At most one pending UI delivery; slow rendering cannot accumulate callbacks.
		fyne.DoAndWait(func() {
			if u.ctx.Err() != nil || u.taskID != selection.id || u.taskRevision != selection.revision {
				return
			}
			states := map[string]string{"blocked": "需要调整", "ready": "就绪", "running": "运行中", "awaiting_approval": "等待确认", "interrupted": "已暂停", "failed": "执行失败", "completed": "验证完成", "needs_verification": "等待验证"}
			u.taskStatus.SetText(states[update.status])
			if u.taskStatus.Text != "" {
				u.taskStatus.Show()
			} else {
				u.taskStatus.Hide()
			}
			if u.conversationView != nil {
				u.conversationView.update(update.id, update.messages)
				if update.status == "running" {
					u.stopAction.Show()
				} else {
					u.stopAction.Hide()
				}
				if update.status == "awaiting_approval" {
					u.approvalAction.Show()
				} else {
					u.approvalAction.Hide()
				}
			} else if u.taskText.Text != update.text {
				u.taskText.SetText(update.text)
			}
		})
	})
}

type liveTaskSource interface {
	taskSource
	SubscribeTask(string) (<-chan struct{}, func())
}

// Coalesce committed deltas at 30 frames per second without polling idle tasks.
func watchTaskUpdates(ctx context.Context, source liveTaskSource, requests <-chan taskSelection, deliver func(taskSelection, taskUpdate)) {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var frame <-chan time.Time
	var notifications <-chan struct{}
	unsubscribe := func() {}
	defer func() { unsubscribe() }()
	var selected taskSelection
	var history taskHistory
	for {
		select {
		case <-ctx.Done():
			return
		case selection, ok := <-requests:
			if !ok {
				return
			}
			selected = selection
			unsubscribe()
			notifications = nil
			timer.Stop()
			frame = nil
			if selected.id != "" {
				notifications, unsubscribe = source.SubscribeTask(selected.id)
			} else {
				unsubscribe = func() {}
			}
			history.loaded = false
		case _, ok := <-notifications:
			if !ok {
				return
			}
			if frame == nil {
				timer.Reset(time.Second / 30)
				frame = timer.C
			}
			continue
		case <-frame:
			frame = nil
		}
		if selected.id == "" {
			history = taskHistory{}
			continue
		}
		update, changed, err := history.load(source, selected.id)
		if err != nil || !changed {
			continue
		}
		deliver(selected, update)
	}
}
