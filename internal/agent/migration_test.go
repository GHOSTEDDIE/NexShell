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

func TestLegacyCheckpointRequiresExplicitMigration(t *testing.T) {
	db, _ := store.Open(t.TempDir())
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	db.Put("models", "test", domain.ModelProfile{ID: "test"})
	s.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &chatModel{}, nil
	}
	legacy := domain.Task{ID: "legacy", Mode: "conversation", ProfileID: "test", Status: "awaiting_approval", Grant: domain.Grant{ID: "old", ExpiresAt: time.Now().Add(time.Hour), Identities: map[string]string{}}}
	db.Put("tasks", legacy.ID, legacy)
	db.Set(context.Background(), legacy.ID, []byte("legacy binary checkpoint"))
	db.Put("history", legacy.ID, []*schema.Message{schema.UserMessage("原始目标"), schema.AssistantMessage("", []schema.ToolCall{{ID: "old-call", Type: "function", Function: schema.FunctionCall{Name: "operate", Arguments: `{}`}}})})
	approval := Approval{Digest: "old-approval", GrantID: "old", Request: domain.Request{TaskID: legacy.ID}}
	db.Put("approvals", approval.Digest, approval)
	if err := s.Submit(legacy.ID, "继续"); err == nil {
		t.Fatal("legacy checkpoint silently resumed")
	}
	if err := s.Decide(legacy.ID, approval.Digest, true); err == nil {
		t.Fatal("legacy approval accepted")
	}
	if err := s.Resume(legacy.ID); err != nil {
		t.Fatal(err)
	}
	current := awaitTask(t, s, legacy.ID, "ready")
	if current.RuntimeVersion != RuntimeVersion || current.Grant.ID == "old" {
		t.Fatal("migration did not renew runtime and grant")
	}
	raw, exists, err := db.Get(context.Background(), legacy.ID)
	if err != nil || !exists || string(raw) != "legacy binary checkpoint" {
		t.Fatal("legacy checkpoint destroyed")
	}
	if checkpointID(current) == legacy.ID {
		t.Fatal("checkpoint namespace unchanged")
	}
}
func TestVerificationRemainsPerHostAndUnknownBlocksCompletion(t *testing.T) {
	db, _ := store.Open(t.TempDir())
	defer db.Close()
	s := NewService(db, nil, nil)
	defer s.Close()
	task := domain.Task{ID: "multi", Grant: domain.Grant{HostIDs: []string{"A", "B"}}}
	for _, host := range []string{"A", "B"} {
		r, _, _ := db.Claim(domain.Request{TaskID: task.ID, HostID: host, CallID: host, Operation: "observe"})
		r.Status = "succeeded"
		r.Output = "healthy"
		db.Finish(r)
	}
	results, _ := db.Results(task.ID)
	if err := s.verifyResult(task, context.Background(), results, results[0].ID, "healthy"); err != nil {
		t.Fatal(err)
	}
	if s.allVerified(task.ID) {
		t.Fatal("one host completed whole task")
	}
	if err := s.verifyResult(task, context.Background(), results, results[1].ID, "healthy"); err != nil {
		t.Fatal(err)
	}
	if !s.allVerified(task.ID) {
		t.Fatal("both verified hosts not complete")
	}
	r, _, _ := db.Claim(domain.Request{TaskID: task.ID, HostID: "A", CallID: "unknown", Operation: "shell"})
	r.Status = "unknown"
	db.Finish(r)
	if s.allVerified(task.ID) || s.checkKnownResults(task.ID) == nil {
		t.Fatal("unknown outcome did not block changes")
	}
}
