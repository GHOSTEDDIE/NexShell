package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"strings"
	"sync"
	"testing"
	"time"
)

func toolMessage(id, name string, args any) *schema.Message {
	b, _ := json.Marshal(args)
	return schema.AssistantMessage("", []schema.ToolCall{{ID: id, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: string(b)}}})
}

type deepScenarioModel struct{}

func (m *deepScenarioModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *deepScenarioModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	child := false
	for _, m := range msgs {
		if m.Role == schema.System && strings.Contains(m.Content, "你是子 Agent，不调用") {
			child = true
		}
	}
	results := []*schema.Message{}
	for _, m := range msgs {
		if m.Role == schema.Tool {
			results = append(results, m)
		}
	}
	if child {
		switch len(results) {
		case 0:
			return toolMessage("same-id", "operate", OperationInput{HostID: "lab", Operation: "file_read", Resource: "/config"}), nil
		case 1:
			return toolMessage("change", "operate", OperationInput{HostID: "lab", Operation: "file_write", Resource: "/config", Content: "enabled=true", ExpectedHash: remote.Hash([]byte("enabled=false"))}), nil
		default:
			return schema.AssistantMessage("CHILD_ONLY 已修改配置，请主 Agent 独立验证。", nil), nil
		}
	}
	switch len(results) {
	case 0:
		return toolMessage("plan", "write_todos", map[string]any{"todos": []map[string]string{{"content": "检查并修复配置", "status": "in_progress", "activeForm": "检查配置"}}}), nil
	case 1:
		return toolMessage("skill", "skill", map[string]string{"skill": "builtin:config-change"}), nil
	case 2:
		return toolMessage("delegate", "task", map[string]string{"subagent_type": "operations_worker", "description": "读取并修改 /config 为 enabled=true，返回结果"}), nil
	case 3:
		return toolMessage("same-id", "operate", OperationInput{HostID: "lab", Operation: "file_read", Resource: "/config"}), nil
	case 4:
		var r domain.Result
		if err := json.Unmarshal([]byte(results[len(results)-1].Content), &r); err != nil {
			return nil, err
		}
		return toolMessage("verify", "verify_execution", VerifyInput{ExecutionID: r.ID, ExpectedText: "enabled=true"}), nil
	case 5:
		return toolMessage("plan-done", "write_todos", map[string]any{"todos": []map[string]string{{"content": "检查并修复配置", "status": "completed", "activeForm": "检查配置"}}}), nil
	default:
		return schema.AssistantMessage("配置已独立验证。", nil), nil
	}
}
func (m *deepScenarioModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	if e != nil {
		return nil, e
	}
	return schema.StreamReaderFromArray([]*schema.Message{v}), nil
}

type fixtureExecutor struct {
	s       *store.Store
	mu      sync.Mutex
	content string
	writes  int
}

func (e *fixtureExecutor) Execute(ctx context.Context, r domain.Request) (domain.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	result, fresh, err := e.s.Claim(r)
	if err != nil || !fresh {
		return result, err
	}
	if r.Operation == "file_write" {
		if r.ExpectedHash != remote.Hash([]byte(e.content)) {
			return result, fmt.Errorf("hash conflict")
		}
		e.content = r.Content
		e.writes++
	}
	result.Output = e.content
	result.Status = "succeeded"
	result.ExitCode = 0
	result.FinishedAt = time.Now()
	return result, e.s.Finish(result)
}
func awaitTask(t *testing.T, s *Service, id, want string) domain.Task {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var task domain.Task
		s.Store.Load("tasks", id, &task)
		s.mu.Lock()
		running := s.active[id] != nil
		s.mu.Unlock()
		if task.Status == want && !running {
			return task
		}
		if !running && (task.Status == "failed" || task.Status == "blocked") {
			t.Fatalf("task %s: %s", task.Status, task.Summary)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var task domain.Task
	s.Store.Load("tasks", id, &task)
	t.Fatalf("expected %s, got %s: %s", want, task.Status, task.Summary)
	return task
}
func TestDeepPlanDelegationApprovalEvidence(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Put("hosts", "lab", domain.Host{ID: "lab", Name: "Lab"})
	db.Put("models", "test", domain.ModelProfile{ID: "test"})
	executor := &fixtureExecutor{s: db, content: "enabled=false"}
	svc := NewService(db, executor, nil)
	defer svc.Close()
	svc.SetMemoryEnabled(false)
	svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &deepScenarioModel{}, nil
	}
	task, err := svc.NewTask("修复配置", "test", []string{"lab"}, []string{"file_read"}, []string{"/config"})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Submit(task.ID, task.Goal); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, task.ID, "awaiting_approval")
	approvals, err := store.All[Approval](db, "approvals")
	if err != nil || len(approvals) != 1 {
		t.Fatal(approvals, err)
	}
	a := approvals[0]
	if a.Request.AgentName != "operations_worker" || !strings.Contains(a.Request.RunID, "/child/") {
		t.Fatal("child identity missing", a.Request)
	}
	if executor.writes != 0 {
		t.Fatal("write ran before approval")
	}
	if err = svc.Decide(task.ID, a.Digest, true); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, task.ID, "completed")
	if executor.writes != 1 {
		t.Fatal("write replayed", executor.writes)
	}
	results, _ := db.Results(task.ID)
	if len(results) != 3 {
		t.Fatal("unexpected executions", results)
	}
	if results[0].Request.CallID == results[2].Request.CallID {
		t.Fatal("root and worker IDs collide")
	}
	var history []*schema.Message
	db.Load("history", task.ID, &history)
	for _, m := range history {
		if m.Role == schema.Assistant && strings.Contains(m.Content, "CHILD_ONLY") {
			t.Fatal("child transcript leaked into root history")
		}
	}
	events, _ := db.Events(task.ID, 0)
	kinds := map[string]bool{}
	for _, e := range events {
		kinds[e.Kind] = true
	}
	for _, kind := range []string{"plan_updated", "subtask_state", "verified"} {
		if !kinds[kind] {
			t.Fatal("missing event", kind)
		}
	}
}
