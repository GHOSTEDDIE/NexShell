package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Replay the provider's empty argument string through the real streaming agent
// and tool node, without calling a provider or opening a server connection.
type workspaceContextModel struct {
	arguments  string
	toolName   string
	fragmented bool
}

func (m *workspaceContextModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *workspaceContextModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	for _, msg := range messages {
		if msg.Role == schema.Tool && msg.ToolCallID == "context-call" {
			if strings.Contains(msg.Content, `"is_error":true`) {
				return schema.AssistantMessage("参数未通过校验，未执行操作。", nil), nil
			}
			var result struct {
				Hosts     []json.RawMessage `json:"hosts"`
				Terminals []json.RawMessage `json:"terminals"`
			}
			if err := json.Unmarshal([]byte(msg.Content), &result); err != nil {
				return nil, err
			}
			if result.Hosts == nil || result.Terminals == nil || len(result.Hosts) != 0 || len(result.Terminals) != 0 {
				return nil, fmt.Errorf("unexpected workspace context: %s", msg.Content)
			}
			return schema.AssistantMessage("当前没有授权服务器，可以直接聊天。", nil), nil
		}
	}
	index := 0
	return schema.AssistantMessage("先查看当前会话。", []schema.ToolCall{{Index: &index, ID: "context-call", Type: "function", Function: schema.FunctionCall{Name: m.toolName, Arguments: m.arguments}}}), nil
}
func (m *workspaceContextModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	if m.fragmented && len(msg.ToolCalls) > 0 {
		// The first delta has the call name and empty arguments; later deltas
		// contain the JSON object. Decode only the fully assembled arguments.
		msg.ToolCalls[0].Function.Arguments = ""
		chunks := []*schema.Message{msg}
		index := 0
		for _, part := range []string{"{", "}"} {
			chunks = append(chunks, &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, Function: schema.FunctionCall{Arguments: part}}}})
		}
		return schema.StreamReaderFromArray(chunks), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func TestWorkspaceContextEmptyArgumentsConversation(t *testing.T) {
	for _, tc := range []struct {
		name, arguments, toolName string
		fragmented, wantError     bool
	}{
		{name: "empty", toolName: "workspace_context"},
		{name: "empty_object", arguments: "{}", toolName: "workspace_context"},
		{name: "whitespace", arguments: " \n\t", toolName: "workspace_context"},
		{name: "fragmented_object", arguments: "{}", toolName: "workspace_context", fragmented: true},
		{name: "malformed_json", arguments: "{", toolName: "workspace_context", wantError: true},
		{name: "invalid_field_type", arguments: `{"execution_id":4,"expected_text":true}`, toolName: "verify_execution", wantError: true},
		{name: "required_input_still_rejected", toolName: "operate", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testWorkspaceContextConversation(t, &workspaceContextModel{arguments: tc.arguments, toolName: tc.toolName, fragmented: tc.fragmented}, tc.wantError)
		})
	}
}

func testWorkspaceContextConversation(t *testing.T, response *workspaceContextModel, wantError bool) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Put("models", "default", domain.ModelProfile{ID: "default"}); err != nil {
		t.Fatal(err)
	}
	service := NewService(s, nil, nil)
	defer service.Close()
	service.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return response, nil
	}
	task, err := service.NewConversation("default", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Submit(task.ID, "你好"); err != nil {
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
	if err = s.Load("tasks", task.ID, &task); err != nil {
		t.Fatal(err)
	}
	if wantError {
		if task.Status != "ready" || task.Summary != "参数未通过校验，未执行操作。" {
			t.Fatalf("invalid arguments not returned as feedback: %s %s", task.Status, task.Summary)
		}
		results, err := s.Results(task.ID)
		if err != nil || len(results) != 0 {
			t.Fatalf("invalid tool call executed: %v %v", results, err)
		}
		return
	}

	if task.Status != "ready" {
		t.Fatalf("workspace_context failed: status=%s summary=%s", task.Status, task.Summary)
	}
	var history []*schema.Message
	if err = s.Load("history", task.ID, &history); err != nil {
		t.Fatal(err)
	}
	for _, msg := range history {
		if msg.Role == schema.Assistant && msg.Content == "当前没有授权服务器，可以直接聊天。" {
			return
		}
	}
	t.Fatal("assistant did not continue after workspace_context")
}
