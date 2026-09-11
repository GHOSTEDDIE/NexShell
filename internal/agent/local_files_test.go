package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestLocalGrantAndUploadApproval(t *testing.T) {
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	h := domain.Host{ID: "lab", User: "ops"}
	task := domain.Task{ID: "t", Grant: domain.Grant{ID: "g", HostIDs: []string{h.ID}, Identities: map[string]string{h.ID: domain.Digest(h)}, LocalRoots: []string{root}, ExpiresAt: time.Now().Add(time.Hour)}}
	r := domain.Request{TaskID: task.ID, Operation: "local_read", LocalPath: filepath.Join(root, "config.yml")}
	if ok, e := EvaluateLocal(task, r, time.Now()); !ok || e != nil {
		t.Fatal(ok, e)
	}
	r.LocalPath = root + "-other"
	if ok, e := EvaluateLocal(task, r, time.Now()); ok || e != nil {
		t.Fatal("outside path must require approval", ok, e)
	}
	r.HostID = "lab"
	if _, e := EvaluateLocal(task, r, time.Now()); e == nil {
		t.Fatal("local request carried remote identity")
	}
	r.HostID = ""
	if _, e := EvaluateLocal(task, r, time.Now().Add(2*time.Hour)); e == nil {
		t.Fatal("expired grant accepted")
	}
	r = domain.Request{TaskID: task.ID, HostID: h.ID, Operation: "upload_local", Resource: "/tmp/package.tar.gz", LocalPath: filepath.Join(root, "package.tar.gz"), SourceHash: strings.Repeat("a", 64)}
	if ok, e := Evaluate(task, h, r, time.Now()); ok || e != nil {
		t.Fatal("upload must require exact confirmation", ok, e)
	}
	approval := Approval{Request: r, Digest: domain.Digest(r), GrantID: task.Grant.ID, Decided: true, Approved: true}
	for _, change := range []func(*domain.Request){func(r *domain.Request) { r.LocalPath += "x" }, func(r *domain.Request) { r.SourceHash = strings.Repeat("b", 64) }, func(r *domain.Request) { r.HostID = "other" }, func(r *domain.Request) { r.Resource += "x" }, func(r *domain.Request) { r.UploadMode = "replace" }} {
		changed := r
		change(&changed)
		if ApprovedFor(approval, task, changed) {
			t.Fatal("changed upload inherited approval")
		}
	}
	r.HostID = "other"
	if _, e := Evaluate(task, h, r, time.Now()); e == nil {
		t.Fatal("upload escaped host scope")
	}
}
func TestLocalExecutionUsesRecordsWithoutRemoteVerification(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	p := filepath.Join(root, "config.txt")
	os.WriteFile(p, []byte("deployment fixture"), 0600)
	svc := NewService(s, &remote.Executor{Store: s}, nil)
	defer svc.Close()
	task := domain.Task{ID: "t", Grant: domain.Grant{ID: "g", LocalRoots: []string{root}, ExpiresAt: time.Now().Add(time.Hour)}}
	r := domain.Request{TaskID: task.ID, CallID: "local-read", Operation: "local_read", LocalPath: p}
	output, err := svc.executeTool(context.Background(), task, r)
	if err != nil || !strings.Contains(output, "deployment fixture") {
		t.Fatal(output, err)
	}
	os.WriteFile(p, []byte("changed"), 0600)
	replay, err := svc.executeTool(context.Background(), task, r)
	if err != nil || replay != output {
		t.Fatal("replayed recorded read", err)
	}
	results, err := s.Results(task.ID)
	if err != nil || len(results) != 1 {
		t.Fatal(results, err)
	}
	if svc.hasOperations(task.ID) || svc.allVerified(task.ID) {
		t.Fatal("local evidence counted as remote verification")
	}
	if err = svc.verifyResult(task, context.Background(), results, results[0].ID, "fixture"); err == nil {
		t.Fatal("local read verified remote host")
	}
}

type localReadModel struct{ path string }

func (m *localReadModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *localReadModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			return schema.AssistantMessage("本地文件读取完成。", nil), nil
		}
	}
	return toolMessage("local", "local_files", LocalFilesInput{Operation: "read", Path: m.path}), nil
}
func (m *localReadModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{v}), e
}
func TestDeepLocalReadApprovalResume(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := filepath.Join(t.TempDir(), "deploy.yml")
	os.WriteFile(p, []byte("fixture: deployment"), 0600)
	s.Put("models", "test", domain.ModelProfile{ID: "test"})
	svc := NewService(s, &remote.Executor{Store: s}, nil)
	defer svc.Close()
	svc.SetMemoryEnabled(false)
	svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &localReadModel{path: p}, nil
	}
	task, err := svc.NewConversation("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Submit(task.ID, "读取本地部署配置"); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, task.ID, "awaiting_approval")
	results, _ := s.Results(task.ID)
	if len(results) != 0 {
		t.Fatal("read before approval")
	}
	approvals, err := store.All[Approval](s, "approvals")
	if err != nil || len(approvals) != 1 {
		t.Fatal(approvals, err)
	}
	a := approvals[0]
	if a.Request.HostID != "" || a.Request.LocalPath != func() string { v, _ := localfiles.CanonicalPath(p); return v }() {
		t.Fatal("wrong local request", a.Request)
	}
	if err = svc.Decide(task.ID, a.Digest, true); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, task.ID, "ready")
	results, _ = s.Results(task.ID)
	if len(results) != 1 || !strings.Contains(results[0].Output, "fixture: deployment") {
		t.Fatal("missing local evidence", results)
	}
}

func TestMessageAttachmentGrantsOnlySelectedFile(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dir := t.TempDir()
	p := filepath.Join(dir, "deploy.yml")
	other := filepath.Join(dir, "private.yml")
	os.WriteFile(p, []byte("fixture: deployment"), 0600)
	os.WriteFile(other, []byte("unselected"), 0600)
	s.Put("models", "test", domain.ModelProfile{ID: "test"})
	svc := NewService(s, &remote.Executor{Store: s}, nil)
	defer svc.Close()
	svc.SetMemoryEnabled(false)
	svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
		return &localReadModel{path: p}, nil
	}
	task, err := svc.NewConversation("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.SubmitFiles(task.ID, "分析这个附件", []string{p}); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, svc, task.ID, "ready")
	approvals, _ := store.All[Approval](s, "approvals")
	if len(approvals) != 0 {
		t.Fatal("selected file required redundant approval")
	}
	results, _ := s.Results(task.ID)
	if len(results) != 1 || !strings.Contains(results[0].Output, "fixture: deployment") {
		t.Fatal("missing read evidence")
	}
	real, _ := localfiles.ResolveReference(p)
	if ok, err := svc.attachedFile(task, real); err != nil || !ok {
		t.Fatal(ok, err)
	}
	realOther, _ := localfiles.ResolveReference(other)
	if ok, _ := svc.attachedFile(task, realOther); ok {
		t.Fatal("sibling inherited attachment scope")
	}
	another, err := svc.NewConversation("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.attachedFile(another, real); ok {
		t.Fatal("attachment crossed conversations")
	}
	task.Grant.ID = "renewed"
	if ok, _ := svc.attachedFile(task, real); ok {
		t.Fatal("attachment survived grant replacement")
	}
}
