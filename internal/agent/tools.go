package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
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
		r := domain.Request{TaskID: id, CallID: executionCallID(ctx), RunID: executionRunID(ctx), AgentName: executionAgentName(ctx), HostID: in.HostID, Operation: in.Operation, Resource: in.Resource, Command: in.Command, Content: in.Content, ExpectedHash: in.ExpectedHash, TimeoutSeconds: in.TimeoutSeconds, LocalPath: in.LocalPath, SourceHash: in.SourceHash, UploadMode: in.UploadMode}
		return s.executeTool(ctx, t, r)
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
	local, err := utils.InferTool("local_files", "查询本机绝对路径：stat 获取信息和 SHA-256；list 列目录；read 分段读取 UTF-8 文本。继续读取使用 next_offset。部署包只用 stat，不读取二进制内容。未授权目录请求确认。", func(ctx context.Context, in *LocalFilesInput) (string, error) {
		if in.Operation != "stat" && in.Operation != "list" && in.Operation != "read" {
			return "", correctable("本机文件操作必须为 stat、list 或 read")
		}
		return s.executeTool(ctx, t, domain.Request{TaskID: id, CallID: executionCallID(ctx), RunID: executionRunID(ctx), AgentName: executionAgentName(ctx), Operation: "local_" + in.Operation, LocalPath: in.Path, ReadOffset: in.Offset})
	}, utils.WithUnmarshalArguments(decodeToolArguments[LocalFilesInput]))
	if err != nil {
		return adk.ToolsConfig{}, err
	}
	tools := []tool.BaseTool{op, verify, evidence, local}
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
				LocalRoots []string              `json:"local_roots"`
				Hosts      []hostInfo            `json:"hosts"`
				Terminals  []remote.TerminalInfo `json:"terminals"`
			}{t.Grant.LocalRoots, hosts, terminals})
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

func (s *Service) executeTool(ctx context.Context, t domain.Task, r domain.Request) (string, error) {
	id := t.ID
	var allowed bool
	var err error
	if r.LocalPath != "" {
		r.LocalPath, err = localfiles.CanonicalPath(r.LocalPath)
		if err != nil {
			return "", correctable(err.Error())
		}
	}
	local := localfiles.IsOperation(r.Operation)
	if local {
		allowed, err = EvaluateLocal(t, r, time.Now())
		if err == nil && !allowed {
			allowed, err = s.attachedFile(t, r.LocalPath)
		}
	} else {
		h, e := s.Store.Host(r.HostID)
		if errors.Is(e, sql.ErrNoRows) {
			return "", correctable("服务器编号无效，请用 workspace_context 查看授权主机")
		}
		if e != nil {
			return "", e
		}
		if remote.Mutates(r.Operation) {
			if e := s.checkKnownResults(id); e != nil {
				return "", e
			}
		}
		allowed, err = Evaluate(t, h, r, time.Now())
	}
	if err != nil {
		return "", correctable(err.Error())
	}
	var e error
	if !allowed {
		var approved Approval
		e = s.Store.Load("approvals", domain.Digest(r), &approved)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return "", e
		}
		if e == nil && approved.Decided && approved.GrantID == t.Grant.ID && !approved.Approved {
			return "用户拒绝执行此操作。", nil
		}
		if e != nil || !ApprovedFor(approved, t, r) {
			return "", compose.Interrupt(ctx, &Approval{Request: r, Digest: domain.Digest(r), GrantID: t.Grant.ID, Reason: "操作超出当前自动执行范围，需要确认具体请求"})
		}
	}
	if e = s.event(id, "execution_start", fmt.Sprintf("%s %s %s %s", r.HostID, r.Operation, r.Resource, r.LocalPath)); e != nil {
		return "", e
	}

	guarded := remote.WithAuthority(localfiles.WithRoots(ctx, t.Grant.LocalRoots), r.HostID, t.Grant.Identities[r.HostID], func() error {
		if local {
			_, err := EvaluateLocal(t, r, time.Now())
			return err
		}
		h, err := s.Store.Host(r.HostID)
		if err != nil {
			return err
		}
		_, err = Evaluate(t, h, r, time.Now())
		return err
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
}

// LocalFilesInput exposes only read operations, independently of remote host identity.
type LocalFilesInput struct {
	Operation string `json:"operation" jsonschema:"enum=stat,enum=list,enum=read"`
	Path      string `json:"path" jsonschema:"description=Absolute local file or directory path"`
	Offset    int64  `json:"offset,omitempty" jsonschema:"description=Byte offset for text read; use previous next_offset, default zero"`
}
