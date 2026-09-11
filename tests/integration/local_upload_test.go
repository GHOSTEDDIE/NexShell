//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
)

func testLocalPackageUpload(t *testing.T, ctx context.Context, s *store.Store, m *remote.Manager, h domain.Host) {
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	p := filepath.Join(root, "部署包.tar.gz")
	payload := bytes.Repeat([]byte{0, 1, 2, 255, 4, 5, 6, 7}, 600000)
	if err := os.WriteFile(p, payload, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := localfiles.Inspect(ctx, p, "local_stat")
	if err != nil {
		t.Fatal(err)
	}
	ex := &remote.Executor{Store: s, Manager: m}
	request := domain.Request{TaskID: "local-upload", CallID: "new", HostID: h.ID, Operation: "upload_local", LocalPath: p, SourceHash: info.SHA256, Resource: "/tmp/fixture/deploy.tar.gz"}
	// Identity, transport and exact remote path are checked again during actual dispatch.
	ctx = remote.WithAuthority(localfiles.WithRoots(ctx, []string{root}), h.ID, domain.Digest(h), func() error { return nil })
	result, err := ex.Execute(ctx, request)
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("upload: %+v %v", result, err)
	}
	if len(result.Output) > 512 || strings.Contains(result.Output, string(payload[:100])) {
		t.Fatal("binary bytes leaked into model result")
	}
	again, err := ex.Execute(ctx, request)
	if err != nil || again.ID != result.ID {
		t.Fatal("upload replayed", err)
	}
	proof, err := ex.Execute(ctx, domain.Request{TaskID: request.TaskID, CallID: "verify", HostID: h.ID, Operation: "file_hash", Resource: request.Resource})
	if err != nil || proof.Status != "succeeded" || !strings.Contains(proof.Output, info.SHA256) {
		t.Fatalf("independent hash: %+v %v", proof, err)
	}
	request.CallID = "existing"
	result, err = ex.Execute(ctx, request)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Error, "已存在") {
		t.Fatalf("default overwrote target: %+v %v", result, err)
	}
	os.WriteFile(p, []byte("changed source"), 0600)
	request.CallID = "changed"
	request.UploadMode = "replace"
	result, err = ex.Execute(ctx, request)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Error, "已变化") {
		t.Fatalf("changed source uploaded: %+v %v", result, err)
	}
	hash, _, err := m.FileHash(ctx, h.ID, request.Resource)
	if err != nil || hash != info.SHA256 {
		t.Fatal("failed preflight touched target", hash, err)
	}
	updated, err := localfiles.Inspect(ctx, p, "local_stat")
	if err != nil {
		t.Fatal(err)
	}
	request.CallID = "replace"
	request.SourceHash = updated.SHA256
	result, err = ex.Execute(ctx, request)
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("replace: %+v %v", result, err)
	}
	request.CallID = "resume"
	request.UploadMode = "resume"
	result, err = ex.Execute(ctx, request)
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("resume: %+v %v", result, err)
	}
	entries, _ := os.ReadDir(filepath.Join(s.Dir, "artifacts"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".upload-") {
			t.Fatal("staging leaked")
		}
	}
}

func exerciseLocalDeployment(t *testing.T, ctx context.Context, s *store.Store, m *remote.Manager, h domain.Host, secrets secretMap, real bool) {
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	p := filepath.Join(root, "release.bin")
	os.WriteFile(p, bytes.Repeat([]byte{0, 1, 255, 4}, 2048), 0600)
	info, err := localfiles.Inspect(ctx, p, "local_stat")
	if err != nil {
		t.Fatal(err)
	}
	target := "/tmp/fixture/agent-deploy-" + domain.ID() + ".bin"
	svc := agent.NewService(s, &remote.Executor{Store: s, Manager: m}, secrets)
	defer svc.Close()
	svc.SetMemoryEnabled(false)
	profile := domain.ModelProfile{ID: "deploy-test", Model: "deterministic"}
	if real {
		profile = savedDefaultModel(t, secrets)
	} else {
		svc.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
			return &deployModel{local: p, target: target}, nil
		}
	}
	if err = s.Put("models", profile.ID, profile); err != nil {
		t.Fatal(err)
	}
	goal := fmt.Sprintf("将本机文件 %s 上传到服务器 lab 的 %s，仅新建，不覆盖其他文件。上传后独立验证远端文件与本地一致。不需要解压或执行程序。本次仅验收文件交付。", p, target)
	task, err := svc.NewTask(goal, profile.ID, []string{h.ID}, []string{"file_hash"}, []string{target})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.SubmitFiles(task.ID, goal, []string{p}); err != nil {
		t.Fatal(err)
	}
	for {
		if err = s.Load("tasks", task.ID, &task); err != nil {
			t.Fatal(err)
		}
		if task.Status == "completed" {
			break
		}
		if task.Status == "failed" || task.Status == "blocked" || task.Status == "needs_verification" || task.Status == "interrupted" {
			t.Fatalf("deployment %s: %s", task.Status, task.Summary)
		}
		if task.Status == "awaiting_approval" {
			approvals, err := store.All[agent.Approval](s, "approvals")
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range approvals {
				if a.Request.TaskID != task.ID || a.Decided {
					continue
				}
				r := a.Request
				if r.Operation != "upload_local" || r.HostID != h.ID || r.LocalPath != p || r.Resource != target || r.SourceHash != info.SHA256 || (r.UploadMode != "" && r.UploadMode != "new") {
					svc.Cancel(task.ID)
					t.Fatalf("unexpected deployment approval: %s", r.Operation)
				}
				if err = svc.Decide(task.ID, a.Digest, true); err != nil && !strings.Contains(err.Error(), "仍在运行") {
					t.Fatal(err)
				}
			}
		}
		select {
		case <-ctx.Done():
			svc.Cancel(task.ID)
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	results, err := s.Results(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	uploaded, verified := false, false
	for _, r := range results {
		uploaded = uploaded || (r.Request.Operation == "upload_local" && r.Status == "succeeded")
		verified = verified || (r.Request.Operation == "file_hash" && r.Request.AgentName == "operator" && r.Status == "succeeded" && strings.Contains(r.Output, info.SHA256))
	}
	if !uploaded || !verified {
		t.Fatal("missing deployment or independent evidence")
	}
	hash, _, err := m.FileHash(ctx, h.ID, target)
	if err != nil || hash != info.SHA256 {
		t.Fatal("deployed file mismatch", err)
	}
	t.Logf("deployment real=%t model=%s: local metadata, exact approval, SFTP upload and independent verification passed", real, profile.Model)
}

type deployModel struct{ local, target string }

func (m *deployModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *deployModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	var results []domain.Result
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			var r domain.Result
			json.Unmarshal([]byte(msg.Content), &r)
			results = append(results, r)
		}
	}
	switch len(results) {
	case 0:
		return deepCall("stat", "local_files", agent.LocalFilesInput{Operation: "stat", Path: m.local}), nil
	case 1:
		var info localfiles.Info
		json.Unmarshal([]byte(results[0].Output), &info)
		return deepCall("upload", "operate", agent.OperationInput{HostID: "lab", Operation: "upload_local", Resource: m.target, LocalPath: m.local, SourceHash: info.SHA256}), nil
	case 2:
		return deepCall("hash", "operate", agent.OperationInput{HostID: "lab", Operation: "file_hash", Resource: m.target}), nil
	case 3:
		return deepCall("verify", "verify_execution", agent.VerifyInput{ExecutionID: results[2].ID, ExpectedText: results[2].Output}), nil
	default:
		return schema.AssistantMessage("部署包已上传并校验。", nil), nil
	}
}
func (m *deployModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	v, e := m.Generate(ctx, msgs, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{v}), e
}
