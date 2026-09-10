package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"io"
	"os"
	"strings"
	"time"
)

func (s *Service) buildTools(ctx context.Context, t domain.Task) (adk.ToolsConfig, error) {
	id := t.ID
	op, err := utils.InferTool("operate", "在明确指定的服务器上观察或执行操作。shell 总是要求本次请求授权；文件修改需要之前读取的哈希。", func(ctx context.Context, in *OperationInput) (string, error) {
		r := domain.Request{TaskID: id, CallID: executionCallID(ctx), RunID: executionRunID(ctx), AgentName: executionAgentName(ctx), HostID: in.HostID, Operation: in.Operation, Resource: in.Resource, Command: in.Command, Content: in.Content, ExpectedHash: in.ExpectedHash, TimeoutSeconds: in.TimeoutSeconds}
		h, e := s.Store.Host(r.HostID)
		if errors.Is(e, sql.ErrNoRows) {
			return "", correctable("服务器编号无效，请先用 workspace_context 查看授权范围并使用返回的 host_id。")
		}
		if e != nil {
			return "", e
		}
		if remote.Mutates(r.Operation) {
			if e := s.checkKnownResults(id); e != nil {
				return "", e
			}
		}
		allowed, e := Evaluate(t, h, r, time.Now())
		if e != nil {
			return "", correctable(e.Error())
		}
		if !allowed {
			var approved Approval
			e = s.Store.Load("approvals", domain.Digest(r), &approved)
			if e == nil && approved.Decided && approved.GrantID == t.Grant.ID && !approved.Approved {
				return "用户拒绝执行此操作。", nil
			}
			if e != nil || !ApprovedFor(approved, t, r) {
				return "", compose.Interrupt(ctx, &Approval{Request: r, Digest: domain.Digest(r), GrantID: t.Grant.ID, Reason: "操作超出当前自动执行范围，需要确认具体请求"})
			}
		}
		if e = s.event(id, "execution_start", fmt.Sprintf("%s %s %s", r.HostID, r.Operation, r.Resource)); e != nil {
			return "", e
		}

		guarded := remote.WithAuthority(ctx, r.HostID, t.Grant.Identities[r.HostID], func() error {
			h, e := s.Store.Host(r.HostID)
			if e != nil {
				return e
			}
			_, e = Evaluate(t, h, r, time.Now())
			return e
		})
		result, e := s.Executor.Execute(guarded, r)
		if e != nil {
			return "", e
		}
		if e = s.event(id, "execution_result", fmt.Sprintf("%s %s exit=%d\n%s", result.ID, result.Status, result.ExitCode, result.Output)); e != nil {
			return "", e
		}
		// Persisted results are not a replay instruction. Unknown outcomes halt the turn.
		if result.Status == "unknown" || result.Status == "running" {
			return "", errors.New("远端结果未确认，停止自动执行；请人工核实执行记录")
		}
		b, _ := json.Marshal(result)
		return string(b), nil
	}, utils.WithUnmarshalArguments(decodeToolArguments[OperationInput]))
	if err != nil {
		return adk.ToolsConfig{}, err
	}
	verify, _ := utils.InferTool("verify_execution", "引用最后一次只读检查的执行编号和预期文本验证结果；有未确认变更时不能完成任务。", func(ctx context.Context, in *VerifyInput) (string, error) {
		if in.ExpectedText == "" {
			return "", correctable("必须提供要核对的预期内容")
		}
		results, e := s.Store.Results(id)
		if e != nil {
			return "", e
		}
		if e = s.verifyResult(t, ctx, results, in.ExecutionID, in.ExpectedText); e != nil {
			return "", e
		}
		return "验证通过", s.event(id, "verified", in.ExecutionID+": "+in.ExpectedText)
	}, utils.WithUnmarshalArguments(decodeToolArguments[VerifyInput]))
	evidence, _ := utils.InferTool("read_evidence", "读取本任务的完整执行证据，每次最多 16 KiB。", func(ctx context.Context, in *EvidenceInput) (string, error) {
		if in.Offset < 0 {
			return "", correctable("offset cannot be negative")
		}
		results, e := s.Store.Results(id)
		if e != nil {
			return "", e
		}
		for _, r := range results {
			if r.ID == in.ExecutionID {
				f, e := os.Open(r.Artifact)
				if e != nil {
					return "", e
				}
				defer f.Close()
				if _, e = f.Seek(in.Offset, io.SeekStart); e != nil {
					return "", e
				}
				b, e := io.ReadAll(io.LimitReader(f, 16384))
				return string(b), e
			}
		}
		return "", correctable("证据不属于本任务")
	}, utils.WithUnmarshalArguments(decodeToolArguments[EvidenceInput]))
	tools := []tool.BaseTool{op, verify, evidence}
	{
		contextTool, err := utils.InferTool("workspace_context", "列出本次对话授权的服务器及仍打开的终端。active 表示当前标签，不是授权。操作必须使用明确编号。", func(ctx context.Context, _ *struct{}) (string, error) {
			type hostInfo struct {
				ID      string `json:"host_id"`
				Name    string `json:"name"`
				Address string `json:"address"`
				User    string `json:"user"`
			}
			hosts := []hostInfo{}
			for _, id := range t.Grant.HostIDs {
				h, e := s.Store.Host(id)
				if e != nil {
					return "", e
				}
				hosts = append(hosts, hostInfo{h.ID, h.Name, h.Address, h.User})
			}
			terminals := []remote.TerminalInfo{}
			if s.Desktop != nil {
				terminals = s.Desktop.List(t.Grant.HostIDs)
			}
			b, e := json.Marshal(struct {
				Hosts     []hostInfo            `json:"hosts"`
				Terminals []remote.TerminalInfo `json:"terminals"`
			}{hosts, terminals})
			return string(b), e
		}, utils.WithUnmarshalArguments(unmarshalNoArguments))
		if err != nil {
			return adk.ToolsConfig{}, err
		}
		tools = append(tools, contextTool)
	}
	budgetCorrections := &correctionBudget{}
	toolNames := []string{}
	for i, base := range tools {
		invokable, ok := base.(tool.InvokableTool)
		if !ok {
			return adk.ToolsConfig{}, fmt.Errorf("工具不支持调用")
		}
		info, err := base.Info(ctx)
		if err != nil {
			return adk.ToolsConfig{}, err
		}
		toolNames = append(toolNames, info.Name)
		tools[i] = &recoveryTool{InvokableTool: invokable, name: info.Name, budget: budgetCorrections, record: func(message string) error { return s.event(id, "tool_feedback", message) }}
	}
	unknownTool := &recoveryTool{name: "unknown_tool", budget: budgetCorrections, record: func(message string) error { return s.event(id, "tool_feedback", message) }}
	handleUnknown := func(ctx context.Context, name, input string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		available := append([]string(nil), toolNames...)
		available = append(available, "skill")
		if executionAgentName(ctx) == "operator" {
			available = append(available, "write_todos", "task")
		}
		return unknownTool.feedback(name, "当前应用不提供工具 "+name+"。可用工具："+strings.Join(available, ", ")+"。请使用现有能力，或明确说明无法直接完成；不要虚构执行结果。")
	}

	return adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools, ExecuteSequentially: true, UnknownToolsHandler: handleUnknown}}, nil
}
