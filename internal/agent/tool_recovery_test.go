package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
	"time"
)

type verificationModel struct{ evidence string }

func (m *verificationModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *verificationModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	count := 0
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			count++
			if count == 1 && !strings.Contains(msg.Content, "验证证据不满足条件") {
				return nil, fmt.Errorf("missing corrective feedback: %s", msg.Content)
			}
		}
	}
	if count >= 2 {
		return schema.AssistantMessage("**检查完成**，已使用有效证据重新验证。", nil), nil
	}
	expected := "wrong"
	if count == 1 {
		expected = "healthy"
	}
	b, _ := json.Marshal(VerifyInput{ExecutionID: m.evidence, ExpectedText: expected})
	i := 0
	return schema.AssistantMessage("", []schema.ToolCall{{Index: &i, ID: fmt.Sprint("verify-", count), Type: "function", Function: schema.FunctionCall{Name: "verify_execution", Arguments: string(b)}}}), nil
}
func (m *verificationModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, e := m.Generate(ctx, msgs, opts...)
	if e != nil {
		return nil, e
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}
func TestVerificationErrorSelfCorrects(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.Put("models", "default", domain.ModelProfile{ID: "default"})
	svc := NewService(s, nil, nil)
	defer svc.Close()
	s.Put("hosts", "lab", domain.Host{ID: "lab", Name: "Fixture"})
	task, e := svc.NewConversation("default", []string{"lab"})
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := s.Claim(domain.Request{TaskID: task.ID, CallID: "observe", HostID: "lab", Operation: "observe"})
	if e != nil {
		t.Fatal(e)
	}
	r.Status = "succeeded"
	r.Output = "healthy"
	s.Finish(r)
	svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &verificationModel{evidence: r.ID}, nil
	}
	if e = svc.Submit(task.ID, "检查运行情况"); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		svc.mu.Lock()
		running := svc.active[task.ID] != nil
		svc.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.Load("tasks", task.ID, &task)
	if task.Status == "failed" || !strings.Contains(task.Summary, "检查完成") {
		t.Fatalf("verification aborted instead of self-correcting: %s %s", task.Status, task.Summary)
	}
	events, e := s.Events(task.ID, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, ev := range events {
		if ev.Kind == "verified" {
			return
		}
	}
	t.Fatal("no real verified event after correction")
}

func TestRecoveryPreservesFatalErrorsAndStopsRepeating(t *testing.T) {
	fatal := errors.New("storage unavailable")
	base, err := utils.InferTool("check", "check", func(context.Context, *struct{}) (string, error) { return "", fatal })
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &recoveryTool{InvokableTool: base, name: "check"}
	if _, err = wrapped.InvokableRun(context.Background(), "{}"); !errors.Is(err, fatal) {
		t.Fatalf("fatal error swallowed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = wrapped.InvokableRun(ctx, "{}"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation swallowed")
	}
	for i := 0; i < 3; i++ {
		out, err := wrapped.InvokableRun(context.Background(), "bad-json")
		if err != nil {
			t.Fatal(err)
		}
		var feedback struct {
			IsError   bool `json:"is_error"`
			Retryable bool `json:"retryable"`
		}
		if err = json.Unmarshal([]byte(out), &feedback); err != nil {
			t.Fatal(err)
		}
		if !feedback.IsError || feedback.Retryable != (i < 2) {
			t.Fatalf("incorrect recovery budget: %s", out)
		}
	}
}
