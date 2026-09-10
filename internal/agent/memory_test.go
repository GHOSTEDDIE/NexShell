package agent

import (
	"context"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/adk"
	fs "github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryIsolationDeletionDisableAndCancellation(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	a, _ := s.newMemoryBackend("host:A")
	b, _ := s.newMemoryBackend("host:B")
	ctx := context.Background()
	name := filepath.Join(a.root, "MEMORY.md")
	if err = a.Write(ctx, &fs.WriteRequest{FilePath: name, Content: "服务器 A 的已验证事实"}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Read(ctx, &fs.ReadRequest{FilePath: name}); err == nil {
		t.Fatal("cross-host path accepted")
	}
	if _, err = b.Read(ctx, &fs.ReadRequest{FilePath: "MEMORY.md"}); err == nil {
		t.Fatal("server A memory leaked to B")
	}
	if err = a.Write(ctx, &fs.WriteRequest{FilePath: filepath.Join(a.root, "..", "outside.md"), Content: "escape"}); err == nil {
		t.Fatal("escaped root")
	}
	if err = a.Write(ctx, &fs.WriteRequest{FilePath: name, Content: "api_key=do-not-store"}); err == nil {
		t.Fatal("credential persisted")
	}
	if err = s.DeleteMemory("host:A", "MEMORY.md"); err != nil {
		t.Fatal(err)
	}
	if err = a.Write(ctx, &fs.WriteRequest{FilePath: name, Content: "stale async write"}); err == nil {
		t.Fatal("deleted memory resurrected")
	}
	a, _ = s.newMemoryBackend("host:A")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = a.Write(cancelled, &fs.WriteRequest{FilePath: name, Content: "cancelled"}); err == nil {
		t.Fatal("cancelled write accepted")
	}
	s.SetMemoryEnabled(false)
	if err = a.Write(ctx, &fs.WriteRequest{FilePath: name, Content: "disabled"}); err == nil {
		t.Fatal("disabled write accepted")
	}
}

type extractionModel struct{ root string }

func (m *extractionModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *extractionModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			return schema.AssistantMessage("记忆已保存。", nil), nil
		}
	}
	return toolMessage("memory-write", "write_file", map[string]string{"file_path": filepath.Join(m.root, "MEMORY.md"), "content": "用户偏好简洁中文回复。"}), nil
}
func (m *extractionModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{v}), e
}
func TestOfficialAutoMemoryWritesAndLoadsAcrossConversations(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	b, _ := s.newMemoryBackend("preferences")
	h, err := s.memoryMiddleware(context.Background(), domain.Task{ID: "first"}, &extractionModel{root: b.root})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.AfterAgent(context.Background(), &adk.ChatModelAgentState{Messages: []*schema.Message{schema.UserMessage("以后请用简洁中文回复")}})
	if err != nil {
		t.Fatal(err)
	}
	records, err := s.ListMemory("preferences")
	if err != nil || len(records) != 1 {
		events, _ := db.Events("first", 0)
		t.Fatal(records, err, events)
	}
	next, err := s.memoryMiddleware(context.Background(), domain.Task{ID: "second"}, &chatModel{})
	if err != nil {
		t.Fatal(err)
	}
	_, run, err := next.BeforeAgent(context.Background(), &adk.ChatModelAgentContext[*schema.Message]{AgentInput: &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("你好")}}})
	if err != nil {
		t.Fatal(err)
	}
	found := strings.Contains(run.Instruction, "简洁中文")
	for _, msg := range run.AgentInput.Messages {
		found = found || strings.Contains(msg.Content, "简洁中文")
	}
	if !found {
		t.Fatal("memory not injected into new conversation")
	}
}
func TestMemoryInputExcludesRawOutputAndOtherHosts(t *testing.T) {
	db, _ := store.Open(t.TempDir())
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	task := domain.Task{ID: "t", Grant: domain.Grant{HostIDs: []string{"A"}}}
	r, _, _ := db.Claim(domain.Request{TaskID: "t", CallID: "c", HostID: "A", Operation: "file_read", Resource: "/health"})
	r.Status = "succeeded"
	r.Output = "raw-do-not-store"
	db.Finish(r)
	db.Put("verification:t", "A", Verification{ExecutionID: r.ID, ExpectedText: "healthy"})
	text, err := s.memoryInput(task, "host:A", nil)
	if err != nil || !strings.Contains(text, "healthy") || strings.Contains(text, "raw-do-not-store") {
		t.Fatal(text, err)
	}
	text, err = s.memoryInput(task, "host:B", nil)
	if err != nil || text != "" {
		t.Fatal("cross-host extraction", text, err)
	}
}

type memorySelectorModel struct{ chatModel }

func (m *memorySelectorModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *memorySelectorModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return toolMessage("select", "select_memories", map[string]any{"selected_memories": []string{"health.md"}}), nil
}
func TestOfficialMemoryReadsIndexedTopic(t *testing.T) {
	db, _ := store.Open(t.TempDir())
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	b, _ := s.newMemoryBackend("host:A")
	ctx := context.Background()
	b.Write(ctx, &fs.WriteRequest{FilePath: "MEMORY.md", Content: "[健康状态](health.md)"})
	b.Write(ctx, &fs.WriteRequest{FilePath: "health.md", Content: "---\nname: health\ndescription: 已验证的健康标记\ntype: project\n---\n主机 A 健康标记是 deep-agent-healthy。"})
	h, err := s.memoryMiddleware(ctx, domain.Task{ID: "next", Grant: domain.Grant{HostIDs: []string{"A"}}}, &memorySelectorModel{})
	if err != nil {
		t.Fatal(err)
	}
	_, run, err := h.BeforeAgent(ctx, &adk.ChatModelAgentContext[*schema.Message]{AgentInput: &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("健康标记是什么")}}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range run.AgentInput.Messages {
		found = found || strings.Contains(msg.Content, "deep-agent-healthy")
	}
	if !found {
		t.Fatal("topic body was not injected")
	}
}
