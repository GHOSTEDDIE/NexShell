package agent

import (
	"context"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"testing"
	"time"
)

type chatModel struct{}

func (m *chatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) { return m, nil }
func (m *chatModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("可以一起查看问题。", nil), nil
}
func (m *chatModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{v}), e
}
func TestConversationWithoutServerContinues(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Put("models", "model", domain.ModelProfile{ID: "model"})
	service := NewService(s, nil, nil)
	defer service.Close()
	service.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &chatModel{}, nil
	}
	task, err := service.NewConversation("model", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"你好", "继续解释"} {
		if err = service.Submit(task.ID, text); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			service.mu.Lock()
			running := service.active[task.ID] != nil
			service.mu.Unlock()
			if !running {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("conversation did not settle")
			}
			time.Sleep(10 * time.Millisecond)
		}
		s.Load("tasks", task.ID, &task)
		if task.Status != "ready" {
			t.Fatal(task.Status, task.Summary)
		}
	}
	var history []*schema.Message
	s.Load("history", task.ID, &history)
	users := 0
	for _, m := range history {
		if m.Role == schema.User {
			users++
		}
	}
	if users != 2 {
		t.Fatalf("lost conversational history: %d", users)
	}
}
func TestTerminalApprovalCannotRetarget(t *testing.T) {
	h := domain.Host{ID: "A"}
	task := domain.Task{ID: "task", Grant: domain.Grant{ID: "g", HostIDs: []string{"A"}, Identities: map[string]string{"A": domain.Digest(h)}, Operations: []string{"terminal_write"}, ExpiresAt: time.Now().Add(time.Hour)}}
	r := domain.Request{TaskID: "task", HostID: "A", Operation: "terminal_write", Resource: "session1", Content: "pwd\n"}
	if ok, err := Evaluate(task, h, r, time.Now()); ok || err != nil {
		t.Fatal("terminal input inherited blanket approval", err)
	}
	approval := Approval{Request: r, Digest: domain.Digest(r), GrantID: "g", Approved: true, Decided: true}
	r.Resource = "session2"
	if ApprovedFor(approval, task, r) {
		t.Fatal("approval changed terminal")
	}
	r = approval.Request
	r.Content = "rm file\n"
	if ApprovedFor(approval, task, r) {
		t.Fatal("approval changed input")
	}
}
