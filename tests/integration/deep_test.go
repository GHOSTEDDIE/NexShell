//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const deepCommand = "printf 'deep-agent-healthy\\n' > /tmp/fixture/deep-agent-health.txt\n"

func exerciseDeepTerminal(t *testing.T, ctx context.Context, m *remote.Manager, s *store.Store, secrets secretMap, real bool) {
	t.Helper()
	desktop := &remote.DesktopTerminals{}
	desktop.Open = func(ctx context.Context, host string) (string, error) {
		session, err := m.Terminal(ctx, host, 24, 80)
		if err != nil {
			return "", err
		}
		t.Cleanup(session.Close)
		core := terminal.NewCore(80, 24)
		t.Cleanup(func() { core.CloseInput() })
		go io.Copy(io.Discard, core)
		go io.Copy(core, session.Output)
		id := desktop.Register(host, session.Input, func() string { return strings.Join(core.Lines(), "\n") }, func() bool { return true })
		return id, nil
	}
	ex := &remote.Executor{Manager: m, Store: s, Desktop: desktop}
	svc := agent.NewService(s, ex, secrets)
	svc.Desktop = desktop
	defer svc.Close()
	profile := domain.ModelProfile{ID: "deep-test", Model: "deterministic", ContextTokens: 32000}
	if real {
		root, _ := os.UserConfigDir()
		database, err := sql.Open("sqlite", "file:"+filepath.Join(root, "NexShell", "state.db")+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		var raw []byte
		if err = database.QueryRow("SELECT payload FROM documents WHERE kind=? AND id=?", "models", "default").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &profile); err != nil {
			t.Fatal(err)
		}
		key, err := (store.Credentials{}).Get("model:" + profile.ID)
		if err != nil {
			t.Fatal(err)
		}
		secrets["model:"+profile.ID] = key
		svc.ModelFactory = func(ctx context.Context, p domain.ModelProfile, secrets remote.Secrets) (model.ToolCallingChatModel, error) {
			cm, err := agent.NewModel(ctx, p, secrets)
			if err != nil {
				return nil, err
			}
			return &diagnosticModel{ToolCallingChatModel: cm, t: t}, nil
		}
	} else {
		svc.SetMemoryEnabled(false)
		svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
			return &deepTerminalModel{}, nil
		}
	}
	if err := s.Put("models", profile.ID, profile); err != nil {
		t.Fatal(err)
	}
	goal := "这是隔离验收，只允许 lab 服务器。必须使用 write_todos 制定计划，并使用 task 委派 operations_worker。子 Agent 用 operate terminal_connect 打开终端，再向该终端 terminal_write 发送以下精确内容（包含末尾换行），不要使用其他写入方式：\n" + deepCommand + "\n子 Agent 可用 terminal_read 观察输出，然后返回。主 Agent 必须独立用 file_read 读取 /tmp/fixture/deep-agent-health.txt，并以 deep-agent-healthy 验证，最后更新计划完成。请将已验证的健康标记作为该主机的长期事实记住。"
	task, err := svc.NewTask(goal, profile.ID, []string{"lab"}, []string{"terminal_connect", "terminal_read", "file_read"}, []string{"/tmp/fixture/deep-agent-health.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Submit(task.ID, goal); err != nil {
		t.Fatal(err)
	}
	awaitDeepCompletion(t, ctx, svc, task.ID)
	results, err := s.Results(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	childWrite, rootRead := false, false
	for _, r := range results {
		if r.Request.Operation == "terminal_write" && r.Request.AgentName == "operations_worker" && r.Status == "succeeded" {
			childWrite = true
		}
		if r.Request.Operation == "file_read" && r.Request.AgentName == "operator" && r.Status == "succeeded" {
			rootRead = true
		}
	}
	if !childWrite || !rootRead {
		t.Fatal("missing child terminal write or independent root read")
	}
	events, _ := s.Events(task.ID, 0)
	planned, delegated := false, false
	for _, e := range events {
		planned = planned || e.Kind == "plan_updated"
		delegated = delegated || e.Kind == "subtask_state"
	}
	if !planned || !delegated {
		t.Fatal("missing deep planning/delegation events")
	}
	if real {
		memories, err := svc.ListMemory("host:lab")
		if err != nil || len(memories) == 0 {
			t.Fatal("real model did not persist verified host memory", err)
		}
		convo, err := svc.NewConversation(profile.ID, []string{"lab"})
		if err != nil {
			t.Fatal(err)
		}
		if err = svc.Submit(convo.ID, "仅根据记忆回答这台服务器上一次已验证的健康标记是什么。不要连接服务器或调用任何远端操作，也无需更新记忆。"); err != nil {
			t.Fatal(err)
		}
		awaitDeepCompletion(t, ctx, svc, convo.ID)
		var done domain.Task
		s.Load("tasks", convo.ID, &done)
		if !strings.Contains(done.Summary, "deep-agent-healthy") {
			t.Fatalf("real model failed cross-conversation recall; summary=%s memories=%+v", done.Summary, memories)
		}
		t.Log("REAL_MODEL: planning, delegation, exact terminal approval, independent verification, host memory and new conversation recall passed")
	}
}
func awaitDeepCompletion(t *testing.T, ctx context.Context, s *agent.Service, id string) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task domain.Task
		if err := s.Store.Load("tasks", id, &task); err != nil {
			t.Fatal(err)
		}
		switch task.Status {
		case "completed", "ready":
			return
		case "failed", "blocked", "interrupted", "needs_verification":
			t.Fatalf("deep task %s: %s", task.Status, task.Summary)
		case "awaiting_approval":
			approvals, err := store.All[agent.Approval](s.Store, "approvals")
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range approvals {
				if a.Request.TaskID != id || a.Decided {
					continue
				}
				r := a.Request
				if r.HostID != "lab" || r.Operation != "terminal_write" || r.Content != deepCommand || r.AgentName != "operations_worker" {
					s.Cancel(id)
					t.Fatalf("unexpected approval scope: operation=%s actor=%s", r.Operation, r.AgentName)
				}
				// The test authorizes only the literal disposable-fixture write above.
				err = s.Decide(id, a.Digest, true)
				if err != nil && !strings.Contains(err.Error(), "仍在运行") {
					t.Fatal(err)
				}
			}
		}
		select {
		case <-ctx.Done():
			s.Cancel(id)
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

type deepTerminalModel struct{}

func (m *deepTerminalModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func deepCall(id, name string, args any) *schema.Message {
	b, _ := json.Marshal(args)
	return schema.AssistantMessage("", []schema.ToolCall{{ID: id, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: string(b)}}})
}
func (m *deepTerminalModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	child := false
	results := []*schema.Message{}
	for _, m := range msgs {
		if m.Role == schema.System && strings.Contains(m.Content, "你是子 Agent，不调用") {
			child = true
		}
		if m.Role == schema.Tool {
			results = append(results, m)
		}
	}
	if child {
		switch len(results) {
		case 0:
			return deepCall("connect", "operate", agent.OperationInput{HostID: "lab", Operation: "terminal_connect"}), nil
		case 1:
			var r domain.Result
			if err := json.Unmarshal([]byte(results[0].Content), &r); err != nil {
				return nil, err
			}
			var info remote.TerminalInfo
			if err := json.Unmarshal([]byte(r.Output), &info); err != nil {
				return nil, err
			}
			return deepCall("write", "operate", agent.OperationInput{HostID: "lab", Operation: "terminal_write", Resource: info.ID, Content: deepCommand}), nil
		default:
			return schema.AssistantMessage("已发送指定健康标记写入，请主 Agent 核实。", nil), nil
		}
	}
	switch len(results) {
	case 0:
		return deepCall("plan", "write_todos", map[string]any{"todos": []map[string]string{{"content": "验证健康标记", "status": "in_progress", "activeForm": "验证健康标记"}}}), nil
	case 1:
		return deepCall("delegate", "task", map[string]string{"subagent_type": "operations_worker", "description": "连接 lab 并写入健康标记"}), nil
	case 2:
		return deepCall("read", "operate", agent.OperationInput{HostID: "lab", Operation: "file_read", Resource: "/tmp/fixture/deep-agent-health.txt"}), nil
	case 3:
		var r domain.Result
		if err := json.Unmarshal([]byte(results[2].Content), &r); err != nil {
			return nil, fmt.Errorf("root result: %w", err)
		}
		return deepCall("verify", "verify_execution", agent.VerifyInput{ExecutionID: r.ID, ExpectedText: "deep-agent-healthy"}), nil
	case 4:
		return deepCall("plan-done", "write_todos", map[string]any{"todos": []map[string]string{{"content": "验证健康标记", "status": "completed", "activeForm": "验证健康标记"}}}), nil
	default:
		return schema.AssistantMessage("已验证健康标记 deep-agent-healthy。", nil), nil
	}
}
func (m *deepTerminalModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	if e != nil {
		return nil, e
	}
	return schema.StreamReaderFromArray([]*schema.Message{v}), nil
}

type diagnosticModel struct {
	model.ToolCallingChatModel
	t *testing.T
}

func (m *diagnosticModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.ToolCallingChatModel.WithTools(infos)
	if err != nil {
		return nil, err
	}
	return &diagnosticModel{bound, m.t}, nil
}
func (m *diagnosticModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := m.ToolCallingChatModel.Stream(ctx, msgs, opts...)
	if err != nil {
		m.t.Logf("MODEL_ESTABLISH_ERROR: %T %v", err, err)
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer stream.Close()
		defer writer.Close()
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				m.t.Logf("MODEL_STREAM_ERROR: %T %v", err, err)
				writer.Send(nil, err)
				return
			}
			if writer.Send(chunk, nil) {
				return
			}
		}
	}()
	return reader, nil
}
