package agent

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func init() {
	gob.RegisterName("nexshell.approval.v1", &Approval{})
	gob.RegisterName("nexshell.input.v1", Input{})
}

type Input struct {
	Text       string
	ApprovalID string
}
type ExecutionService interface {
	Execute(context.Context, domain.Request) (domain.Result, error)
}
type activeRun struct {
	loop   *adk.TurnLoop[Input, *schema.Message]
	cancel context.CancelFunc
	done   chan struct{}
}
type Service struct {
	memoryMu     sync.RWMutex
	Desktop      *remote.DesktopTerminals
	Store        *store.Store
	Executor     ExecutionService
	Secrets      remote.Secrets
	ModelFactory func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error)
	mu           sync.Mutex
	active       map[string]*activeRun
	closed       bool
}

func NewService(s *store.Store, e ExecutionService, secrets remote.Secrets) *Service {
	return &Service{Store: s, Executor: e, Secrets: secrets, ModelFactory: NewModel, active: map[string]*activeRun{}}
}
func (s *Service) NewTask(goal, profile string, hosts, ops, resources []string) (domain.Task, error) {
	if strings.TrimSpace(goal) == "" || len(hosts) == 0 {
		return domain.Task{}, errors.New("请选择服务器并填写运维目标")
	}
	t := domain.Task{RuntimeVersion: RuntimeVersion, ID: domain.ID(), Goal: goal, ProfileID: profile, Status: "ready", CreatedAt: time.Now(), Grant: domain.Grant{ID: domain.ID(), HostIDs: hosts, Operations: ops, Resources: resources, Identities: map[string]string{}, ExpiresAt: time.Now().Add(8 * time.Hour)}}
	for _, id := range hosts {
		h, err := s.Store.Host(id)
		if err != nil {
			return t, err
		}
		t.Grant.Identities[id] = domain.Digest(h)
	}
	return t, s.Store.Put("tasks", t.ID, t)
}
func (s *Service) event(id, kind, text string) error {
	_, err := s.Store.Event(id, kind, text)
	return err
}
func (s *Service) Submit(id, text string) error {
	s.mu.Lock()
	run := s.active[id]
	s.mu.Unlock()
	if run != nil {
		ok, _ := run.loop.Push(Input{Text: text}, adk.WithPreemptTimeout[Input, *schema.Message](adk.AfterToolCalls, 30*time.Second))
		if !ok {
			return errors.New("任务正在停止，请稍后恢复")
		}
		return nil
	}
	var task domain.Task
	if err := s.Store.Load("tasks", id, &task); err != nil {
		return err
	}
	if task.Status == "awaiting_approval" || task.Status == "interrupted" {
		task.Updates = append(task.Updates, text)
		if err := s.Store.Put("tasks", id, task); err != nil {
			return err
		}
	}
	return s.start(id, Input{Text: text})
}
func (s *Service) Resume(id string) error {
	var t domain.Task
	if err := s.Store.Load("tasks", id, &t); err != nil {
		return err
	}
	results, err := s.Store.Results(id)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Status == "unknown" || r.Status == "running" {
			return errors.New("存在结果未确认的执行，请先查看记录并人工核实，使用新任务继续")
		}
	}
	if t.RuntimeVersion != RuntimeVersion {
		t.Status = "interrupted"
	}
	t.RuntimeVersion = RuntimeVersion
	t.Grant.ID = domain.ID()
	t.Grant.ExpiresAt = time.Now().Add(8 * time.Hour)
	for _, host := range t.Grant.HostIDs {
		h, e := s.Store.Host(host)
		if e != nil {
			return e
		}
		t.Grant.Identities[host] = domain.Digest(h)
	}
	if err = s.Store.Put("tasks", id, t); err != nil {
		return err
	}
	return s.start(id, Input{Text: "继续任务，先核实当前状态，禁止重复执行已完成的变更。"})
}
func (s *Service) Decide(id, approvalID string, accept bool) error {
	var a Approval
	if err := s.Store.Load("approvals", approvalID, &a); err != nil {
		return err
	}
	if a.Request.TaskID != id {
		return errors.New("批准不属于此任务")
	}
	var t domain.Task
	if err := s.Store.Load("tasks", id, &t); err != nil {
		return err
	}
	if t.RuntimeVersion != RuntimeVersion || a.GrantID != t.Grant.ID || !time.Now().Before(t.Grant.ExpiresAt) {
		return errors.New("授权已失效")
	}
	a.Approved = accept
	a.Decided = true
	if err := s.Store.Put("approvals", approvalID, a); err != nil {
		return err
	}
	if err := s.event(id, "approval_decision", fmt.Sprintf("%s approved=%t", approvalID, accept)); err != nil {
		return err
	}
	return s.start(id, Input{ApprovalID: approvalID})
}
func (s *Service) Cancel(id string) {
	s.mu.Lock()
	r := s.active[id]
	s.mu.Unlock()
	if r != nil {
		r.loop.Stop(adk.WithImmediate())
		r.cancel()
	} else {
		var t domain.Task
		if s.Store.Load("tasks", id, &t) == nil && (t.Status == "awaiting_approval" || t.Status == "ready") {
			t.Status = "interrupted"
			t.Grant.ExpiresAt = time.Time{}
			_ = s.Store.Put("tasks", id, t)
			_ = s.event(id, "task_state", t.Status)
		}
	}
}
func (s *Service) Stop() {
	s.mu.Lock()
	s.closed = true
	runs := make([]*activeRun, 0, len(s.active))
	for _, r := range s.active {
		runs = append(runs, r)
	}
	s.mu.Unlock()
	for _, r := range runs {
		r.loop.Stop(adk.WithImmediate())
		r.cancel()
	}
}
func (s *Service) Close() {
	s.Stop()
	s.mu.Lock()
	runs := make([]*activeRun, 0, len(s.active))
	for _, r := range s.active {
		runs = append(runs, r)
	}
	s.mu.Unlock()
	for _, r := range runs {
		<-r.done
	}
}

type OperationInput struct {
	HostID         string `json:"host_id" jsonschema:"description=Explicit target host ID"`
	Operation      string `json:"operation" jsonschema:"description=observe service_status logs file_read file_write service_restart package_install shell terminal_connect terminal_read or terminal_write; terminal resource is terminal_id; terminal_write content is exact bytes including newline"`
	Resource       string `json:"resource,omitempty"`
	Command        string `json:"command,omitempty"`
	Content        string `json:"content,omitempty"`
	ExpectedHash   string `json:"expected_hash,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type VerifyInput struct {
	ExecutionID  string `json:"execution_id"`
	ExpectedText string `json:"expected_text"`
}
type EvidenceInput struct {
	ExecutionID string `json:"execution_id"`
	Offset      int64  `json:"offset"`
}
type SkillInput struct {
	Name string `json:"name"`
}

func (s *Service) start(id string, input Input) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("应用正在退出")
	}
	if s.active[id] != nil {
		return errors.New("任务仍在运行")
	}
	var t domain.Task
	if err := s.Store.Load("tasks", id, &t); err != nil {
		return err
	}
	if err := s.prepareVersion(&t); err != nil {
		return err
	}
	if !time.Now().Before(t.Grant.ExpiresAt) {
		return errors.New("请重新确认任务范围后恢复")
	}
	var profile domain.ModelProfile
	if err := s.Store.Load("models", t.ProfileID, &profile); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), executionIdentityKey{}, executionIdentity{RunID: t.ID + "/root", Name: "operator"}))
	cm, err := s.ModelFactory(ctx, profile, s.Secrets)
	if err != nil {
		cancel()
		return err
	}
	var history []*schema.Message
	if e := s.Store.Load("history", id, &history); e != nil && !errors.Is(e, sql.ErrNoRows) {
		cancel()
		return e
	}
	if t.Status == "failed" || t.Status == "interrupted" || t.Status == "blocked" {
		history = completeInterruptedToolCalls(history)
	}
	state := &eventState{}
	var historyMu sync.Mutex
	appendHistory := func(msg *schema.Message) error {
		historyMu.Lock()
		defer historyMu.Unlock()
		history = append(history, msg)
		return s.Store.Put("history", id, history)
	}
	snapshotHistory := func() []*schema.Message {
		historyMu.Lock()
		defer historyMu.Unlock()
		return append([]*schema.Message(nil), history...)
	}
	tools, err := s.buildTools(ctx, t)
	if err != nil {
		cancel()
		return err
	}
	var loop *adk.TurnLoop[Input, *schema.Message]
	loop = adk.NewTurnLoop(adk.TurnLoopConfig[Input, *schema.Message]{
		Store: store.Checkpoints{Store: s.Store}, CheckpointID: checkpointID(t),
		GenInput: func(ctx context.Context, _ *adk.TurnLoop[Input, *schema.Message], items []Input) (*adk.GenInputResult[Input, *schema.Message], error) {
			for _, item := range items {
				if item.Text != "" {
					if e := appendHistory(schema.UserMessage(item.Text)); e != nil {
						return nil, e
					}
					if e := s.event(id, "user", item.Text); e != nil {
						return nil, e
					}
				}
			}

			var todos []deep.TODO
			if e := s.Store.Load("plans", id, &todos); e != nil && !errors.Is(e, sql.ErrNoRows) {
				return nil, e
			}
			return &adk.GenInputResult[Input, *schema.Message]{RunOpts: []adk.AgentRunOption{adk.WithSessionValues(map[string]any{deep.SessionKeyTodos: todos})}, Input: &adk.AgentInput{Messages: snapshotHistory(), EnableStreaming: true}, Consumed: items}, nil
		},
		GenResume: func(ctx context.Context, _ *adk.TurnLoop[Input, *schema.Message], interrupted, unhandled, items []Input) (*adk.GenResumeResult[Input, *schema.Message], error) {
			targets := map[string]any{}
			for _, item := range items {
				if item.ApprovalID != "" {
					var a Approval
					if e := s.Store.Load("approvals", item.ApprovalID, &a); e != nil {
						return nil, e
					}
					targets[a.InterruptID] = a.Approved
				}
			}
			return &adk.GenResumeResult[Input, *schema.Message]{Decision: adk.TurnLoopResumeDecisionResume, ResumeParams: &adk.ResumeParams{Targets: targets}, Consumed: items}, nil
		},
		PrepareAgent: func(ctx context.Context, _ *adk.TurnLoop[Input, *schema.Message], items []Input) (adk.Agent, error) {
			return s.buildAgent(ctx, t, profile, cm, tools, func(messages []*schema.Message) error {
				historyMu.Lock()
				defer historyMu.Unlock()
				history = messages
				return s.Store.Put("history", id, history)
			})
		},
		OnAgentEvents: func(ctx context.Context, tc *adk.TurnContext[Input, *schema.Message], events *adk.AsyncIterator[*adk.AgentEvent]) error {
			return s.consumeEvents(ctx, id, state, events, appendHistory, func() { loop.Stop(adk.UntilIdleFor(20 * time.Millisecond)) })
		},
	})
	run := &activeRun{loop: loop, cancel: cancel, done: make(chan struct{})}
	t.Status = "running"
	if err = s.Store.Put("tasks", id, t); err != nil {
		cancel()
		return err
	}
	s.active[id] = run
	loop.Push(input)
	go func() {
		defer close(run.done)
		defer cancel()
		loop.Run(ctx)
		exit := loop.Wait()
		t.Summary = state.summary
		var blocked *blockedError
		if state.pending {
			t.Status = "awaiting_approval"
		} else if ctx.Err() != nil {
			t.Status = "interrupted"
		} else if errors.As(state.err, &blocked) || errors.As(exit.ExitReason, &blocked) {
			t.Status = "blocked"
			t.Summary = blocked.Error()
		} else if state.err != nil {
			t.Status = "failed"
			t.Summary = state.err.Error()
		} else if exit.ExitReason != nil {
			t.Status = "interrupted"
			t.Summary = exit.ExitReason.Error()
		} else if t.Mode == "conversation" && !s.hasOperations(id) {
			t.Status = "ready"
		} else if s.allVerified(id) {
			t.Status = "completed"
		} else {
			t.Status = "needs_verification"
		}
		s.mu.Lock()
		_ = s.Store.Put("tasks", id, t)
		_ = s.event(id, "task_state", t.Status)
		delete(s.active, id)
		s.mu.Unlock()
	}()
	return nil
}
