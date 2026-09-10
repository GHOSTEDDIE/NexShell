package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

type executionIdentityKey struct{}
type executionIdentity struct{ RunID, Name string }

func executionRunID(ctx context.Context) string {
	v, _ := ctx.Value(executionIdentityKey{}).(executionIdentity)
	return v.RunID
}
func executionAgentName(ctx context.Context) string {
	v, _ := ctx.Value(executionIdentityKey{}).(executionIdentity)
	if v.Name == "" {
		return "operator"
	}
	return v.Name
}
func executionCallID(ctx context.Context) string {
	return executionRunID(ctx) + ":" + compose.GetToolCallID(ctx)
}

type SubtaskEvent struct{ RunID, Agent, State, Message string }
type runtimeObserver struct {
	adk.BaseChatModelAgentMiddleware
	s        *Service
	task     domain.Task
	root     bool
	feedback recoveryTool
}

func (m *runtimeObserver) WrapInvokableToolCall(_ context.Context, next adk.InvokableToolCallEndpoint, tctx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if tctx.Name == "task" {
			var input struct {
				SubagentType string `json:"subagent_type"`
				Description  string `json:"description"`
			}
			if err := json.Unmarshal([]byte(args), &input); err != nil || input.SubagentType != "operations_worker" || input.Description == "" {
				return m.feedback.feedback(args, "task 必须指定 operations_worker 和明确的 description")
			}
		}
		if tctx.Name == "write_todos" {
			var input struct {
				Todos []deep.TODO `json:"todos"`
			}
			if err := json.Unmarshal([]byte(args), &input); err != nil {
				return m.feedback.feedback(args, "计划参数格式错误")
			}
			for _, todo := range input.Todos {
				if todo.Content == "" || (todo.Status != "pending" && todo.Status != "in_progress" && todo.Status != "completed") {
					return m.feedback.feedback(args, "计划必须包含内容和有效状态 pending、in_progress 或 completed")
				}
			}
		}
		if tctx.Name == "task" {
			identity := executionIdentity{RunID: m.task.ID + "/child/" + compose.GetToolCallID(ctx), Name: "operations_worker"}
			if !m.root {
				return "", correctable("子 Agent 不能递归委派")
			}
			ctx = context.WithValue(ctx, executionIdentityKey{}, identity)
			if err := m.emitSubtask(identity, "running", ""); err != nil {
				return "", err
			}
			out, err := next(ctx, args, opts...)
			state := "completed"
			if err != nil {
				state = "interrupted"
				if _, ok := compose.IsInterruptRerunError(err); !ok {
					state = "failed"
				}
			}
			if saveErr := m.emitSubtask(identity, state, out); saveErr != nil {
				return "", saveErr
			}
			return out, err
		}
		out, err := next(ctx, args, opts...)
		if err == nil && tctx.Name == "write_todos" && m.root {
			var input struct {
				Todos []deep.TODO `json:"todos"`
			}
			if e := json.Unmarshal([]byte(args), &input); e != nil {
				return "", e
			}
			if e := m.s.Store.Put("plans", m.task.ID, input.Todos); e != nil {
				return "", e
			}
			b, _ := json.Marshal(input.Todos)
			if e := m.s.event(m.task.ID, "plan_updated", string(b)); e != nil {
				return "", e
			}
		}
		return out, err
	}, nil
}
func (m *runtimeObserver) emitSubtask(id executionIdentity, state, message string) error {
	if len(message) > 8192 {
		message = message[:8192]
	}
	event := SubtaskEvent{id.RunID, id.Name, state, message}
	if err := m.s.Store.Put("subtasks:"+m.task.ID, id.RunID, event); err != nil {
		return err
	}
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("子任务事件: %w", err)
	}
	return m.s.event(m.task.ID, "subtask_state", string(b))
}
