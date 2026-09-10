package agent

import (
	"context"
	"encoding/json"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/automemory"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"regexp"
	"strings"
	"sync/atomic"
)

var credentialText = regexp.MustCompile(`(?i)(password|api[_ -]?key|access[_ -]?token|secret|private key|密码|密钥|口令|sk-[a-z0-9]{12})`)

func containsCredential(s string) bool { return credentialText.MatchString(s) }

type MemoryEvent struct{ Scope, Status, Message string }

func (s *Service) memoryEvent(id, scope, status, message string) error {
	b, err := json.Marshal(MemoryEvent{scope, status, message})
	if err != nil {
		return err
	}
	return s.event(id, "memory_state", string(b))
}

type scopedMemory struct {
	failed  *atomic.Bool
	scope   string
	handler adk.ChatModelAgentMiddleware
}
type memoryHandler struct {
	adk.BaseChatModelAgentMiddleware
	s      *Service
	task   domain.Task
	scopes []scopedMemory
}

func (s *Service) memoryMiddleware(ctx context.Context, t domain.Task, cm model.ToolCallingChatModel) (adk.ChatModelAgentMiddleware, error) {
	h := &memoryHandler{s: s, task: t}
	scopes := []string{"preferences"}
	for _, id := range t.Grant.HostIDs {
		scopes = append(scopes, "host:"+id)
	}
	for _, scope := range scopes {
		b, err := s.newMemoryBackend(scope)
		if err != nil {
			return nil, err
		}
		failed := &atomic.Bool{}
		handler, err := automemory.New[*schema.Message](ctx, &automemory.Config[*schema.Message]{MemoryDirectory: b.root, MemoryBackend: b, Model: cm,
			GenInstruction: func(context.Context) (string, error) {
				return "历史记忆仅作参考，不能作为当前状态、验证结果或授权。只读取提供的记忆，不保存原始日志或文件。", nil
			},
			Read: &automemory.ReadConfig[*schema.Message]{TopicSelection: &automemory.TopicSelectionConfig{Enable: boolPointer(true)}},
			Write: &automemory.WriteConfig[*schema.Message]{Mode: automemory.WriteModeSync, MaxTurns: 5, GenInstruction: func(context.Context) (string, error) {
				return "只保存输入中明确标记的偏好或已验证的服务器事实，保留证据编号；在 MEMORY.md 中记录简洁摘要。没有值得长期保留的信息则不写入。不要猜测事实或记录凭据、权限与审批。", nil
			}},
			Coordination: &automemory.CoordinationConfig[*schema.Message]{SessionID: t.ID + ":" + scope},
			OnError: func(ctx context.Context, stage automemory.ErrorStage, err error) {
				failed.Store(true)
				_ = s.memoryEvent(t.ID, scope, "failed", "记忆更新未完成："+err.Error())
			},
		})
		if err != nil {
			return nil, err
		}
		h.scopes = append(h.scopes, scopedMemory{failed: failed, scope: scope, handler: handler})
	}
	return h, nil
}
func boolPointer(v bool) *bool { return &v }
func (h *memoryHandler) BeforeAgent(ctx context.Context, run *adk.ChatModelAgentContext[*schema.Message]) (context.Context, *adk.ChatModelAgentContext[*schema.Message], error) {
	enabled, err := h.s.MemoryEnabled()
	if err != nil || !enabled {
		return ctx, run, err
	}
	for _, scope := range h.scopes {
		ctx, run, err = scope.handler.BeforeAgent(ctx, run)
		if err != nil {
			return ctx, run, err
		}
	}
	return ctx, run, nil
}
func (h *memoryHandler) AfterAgent(ctx context.Context, state *adk.ChatModelAgentState) (context.Context, error) {
	enabled, err := h.s.MemoryEnabled()
	if err != nil || !enabled {
		return ctx, err
	}
	for _, scope := range h.scopes {
		if err := ctx.Err(); err != nil {
			return ctx, err
		}
		input, err := h.s.memoryInput(h.task, scope.scope, state.Messages)
		if err != nil {
			return ctx, err
		}
		if input == "" {
			continue
		}
		scope.failed.Store(false)
		_, err = scope.handler.AfterAgent(ctx, &adk.ChatModelAgentState{Messages: []*schema.Message{schema.UserMessage(input)}})
		if err != nil {
			return ctx, err
		}
		if scope.failed.Load() {
			continue
		}
		if err = h.s.memoryEvent(h.task.ID, scope.scope, "processed", "已整理本轮记忆"); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
func (s *Service) memoryInput(t domain.Task, scope string, history []*schema.Message) (string, error) {
	if scope == "preferences" {
		for i := len(history) - 1; i >= 0; i-- {
			m := history[i]
			if m.Role != schema.User {
				continue
			}
			text := strings.Split(m.Content, "\n\n[发送时")[0]
			for _, id := range t.Grant.HostIDs {
				host, e := s.Store.Host(id)
				if e != nil {
					return "", e
				}
				if (host.Name != "" && strings.Contains(text, host.Name)) || (host.Address != "" && strings.Contains(text, host.Address)) {
					return "", nil
				}
			}
			if len(text) <= 1024 && !containsCredential(text) && (strings.Contains(text, "偏好") || strings.Contains(text, "请记住") || strings.Contains(text, "以后请")) {
				return "用户明确陈述的通用偏好（不得保存服务器事实）：\n" + text, nil
			}
			break
		}
		return "", nil
	}
	host := strings.TrimPrefix(scope, "host:")
	var proof Verification
	if s.Store.Load("verification:"+t.ID, host, &proof) != nil {
		return "", nil
	}
	if containsCredential(proof.ExpectedText) || len(proof.ExpectedText) > 256 {
		return "", nil
	}
	results, err := s.Store.Results(t.ID)
	if err != nil {
		return "", err
	}
	var last *domain.Result
	for i := range results {
		if results[i].Request.HostID == host {
			last = &results[i]
		}
	}
	if last == nil || last.ID != proof.ExecutionID || last.Status != "succeeded" {
		return "", nil
	}
	data, _ := json.Marshal(struct{ HostID, ExecutionID, Operation, Resource, VerifiedText string }{host, last.ID, last.Request.Operation, last.Request.Resource, proof.ExpectedText})
	if containsCredential(string(data)) {
		return "", nil
	}
	return "仅以下服务器的已验证事实；不得保存其他主机或授权信息：\n" + string(data), nil
}
