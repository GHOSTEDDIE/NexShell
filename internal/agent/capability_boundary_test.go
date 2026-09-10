package agent

import (
	"context"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type boundaryModel struct {
	name, args string
	repeat     bool
	calls      atomic.Int32
}

func (m *boundaryModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *boundaryModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	n := m.calls.Add(1)
	if !m.repeat {
		for _, msg := range msgs {
			if msg.Role == schema.Tool {
				if !strings.Contains(msg.Content, `"is_error":true`) {
					return nil, fmt.Errorf("expected capability feedback: %s", msg.Content)
				}
				return schema.AssistantMessage("当前能力无法直接完成此操作，可以提供步骤或使用已授权的服务器工具。", nil), nil
			}
		}
	}
	i := 0
	return schema.AssistantMessage("", []schema.ToolCall{{Index: &i, ID: fmt.Sprint("call-", n), Type: "function", Function: schema.FunctionCall{Name: m.name, Arguments: m.args}}}), nil
}
func (m *boundaryModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, e := m.Generate(ctx, msgs, opts...)
	if e != nil {
		return nil, e
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func runBoundaryConversation(t *testing.T, m model.ToolCallingChatModel, allowedHosts ...domain.Host) (domain.Task, *store.Store, *Service) {
	t.Helper()
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if e = s.Put("models", "default", domain.ModelProfile{ID: "default"}); e != nil {
		t.Fatal(e)
	}
	svc := NewService(s, nil, nil)
	t.Cleanup(svc.Close)
	svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return m, nil
	}
	var ids []string
	for _, host := range allowedHosts {
		if e = s.Put("hosts", host.ID, host); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, host.ID)
	}
	task, e := svc.NewConversation("default", ids)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Submit(task.ID, "执行超出能力的请求"); e != nil {
		t.Fatal(e)
	}
	waitBoundaryTask(t, s, svc, &task)
	return task, s, svc
}
func waitBoundaryTask(t *testing.T, s *store.Store, svc *Service, task *domain.Task) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		svc.mu.Lock()
		active := svc.active[task.ID] != nil
		svc.mu.Unlock()
		if !active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("assistant did not settle within bounded time")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e := s.Load("tasks", task.ID, task); e != nil {
		t.Fatal(e)
	}
}
func TestCapabilityBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, args string }{
		{"web_search", `{"query":"今日新闻"}`}, {"send_email", `{"to":"nobody@example.invalid"}`}, {"read_local_file", `{"path":"/private/example"}`}, {"browser_open", `{"url":"https://example.invalid"}`}, {"operate", `{"host_id":"unknown","operation":"format_disk"}`}, {"verify_execution", `{"execution_id":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &boundaryModel{name: tc.name, args: tc.args}
			task, s, _ := runBoundaryConversation(t, m)
			if task.Status != "ready" || !strings.Contains(task.Summary, "当前能力") {
				t.Fatalf("capability request aborted: %s %s", task.Status, task.Summary)
			}
			results, e := s.Results(task.ID)
			if e != nil || len(results) != 0 {
				t.Fatalf("out of scope request executed: %v %v", results, e)
			}
		})
	}
}
func TestRepeatedToolErrorsAreBounded(t *testing.T) {
	for _, name := range []string{"web_search", "verify_execution"} {
		t.Run(name, func(t *testing.T) {
			m := &boundaryModel{name: name, args: "{}", repeat: true}
			task, s, _ := runBoundaryConversation(t, m)
			if task.Status != "blocked" || m.calls.Load() > 4 {
				t.Fatalf("unbounded correction loop: %d calls, %s %s", m.calls.Load(), task.Status, task.Summary)
			}
			if r, _ := s.Results(task.ID); len(r) != 0 {
				t.Fatal("repeat loop performed an operation")
			}
		})
	}
}

func TestBlockedConversationCanContinue(t *testing.T) {
	m := &boundaryModel{name: "web_search", args: "{}", repeat: true}
	task, s, svc := runBoundaryConversation(t, m)
	if task.Status != "blocked" {
		t.Fatal(task.Status)
	}
	m.repeat = false
	if err := svc.Submit(task.ID, "那就解释一下操作步骤"); err != nil {
		t.Fatal(err)
	}
	waitBoundaryTask(t, s, svc, &task)
	if task.Status != "ready" {
		t.Fatalf("blocked conversation cannot continue: %s %s", task.Status, task.Summary)
	}
	var history []*schema.Message
	if err := s.Load("history", task.ID, &history); err != nil {
		t.Fatal(err)
	}
	for i, msg := range history {
		if msg.Role != schema.Assistant {
			continue
		}
		for _, call := range msg.ToolCalls {
			found := false
			for j := i + 1; j < len(history) && history[j].Role == schema.Tool; j++ {
				if history[j].ToolCallID == call.ID {
					found = true
				}
			}
			if !found {
				t.Fatalf("unanswered tool call left in resumed history: %s", call.ID)
			}
		}
	}
}

func TestCorrectionBudgetAcrossTools(t *testing.T) {
	budget := &correctionBudget{}
	for i := 0; i < 13; i++ {
		tool := &recoveryTool{name: fmt.Sprint("unknown-", i), budget: budget}
		_, err := tool.feedback(fmt.Sprint(i), "unsupported")
		if (i < 12) != (err == nil) {
			t.Fatalf("cross-tool budget failed at %d: %v", i, err)
		}
	}
}

func TestUnsupportedOperationOnAuthorizedHost(t *testing.T) {
	m := &boundaryModel{name: "operate", args: `{"host_id":"authorized","operation":"format_disk"}`}
	task, s, _ := runBoundaryConversation(t, m, domain.Host{ID: "authorized", Name: "fixture", Address: "127.0.0.1", Port: 1, User: "fixture"})
	if task.Status != "ready" {
		t.Fatalf("unsupported operation aborted: %s %s", task.Status, task.Summary)
	}
	results, e := s.Results(task.ID)
	if e != nil || len(results) != 0 {
		t.Fatal("unsupported operation was executed")
	}
	var history []*schema.Message
	if e = s.Load("history", task.ID, &history); e != nil {
		t.Fatal(e)
	}
	for _, msg := range history {
		if msg.Role == schema.Tool && strings.Contains(msg.Content, "未知工具操作") {
			return
		}
	}
	t.Fatal("test did not reach operation capability validation")
}
