package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
)

type correctableError struct{ message string }

func (e *correctableError) Error() string { return e.message }
func correctable(message string) error    { return &correctableError{message} }

// Only argument mistakes and explicitly marked diagnostic failures become model
// feedback. Cancellation, approval interrupts, storage faults and uncertain
// execution outcomes keep their original control flow.
type recoveryTool struct {
	tool.InvokableTool
	name     string
	record   func(string) error
	mu       sync.Mutex
	attempts map[string]int
	budget   *correctionBudget
}

func (t *recoveryTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized := strings.TrimSpace(args)
	if normalized == "" && t.name == "workspace_context" {
		normalized = "{}"
	}
	if !json.Valid([]byte(normalized)) || !strings.HasPrefix(normalized, "{") {
		return t.feedback(args, "工具参数必须是 JSON 对象，请按工具定义补齐字段。")
	}
	result, err := t.InvokableTool.InvokableRun(ctx, args, opts...)
	var fix *correctableError
	if errors.As(err, &fix) {
		return t.feedback(args, fix.message)
	}
	return result, err
}
func (t *recoveryTool) feedback(args, message string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.attempts == nil {
		t.attempts = map[string]int{}
	}
	var decoded any
	canonical := args
	decoder := json.NewDecoder(strings.NewReader(args))
	decoder.UseNumber()
	if decoder.Decode(&decoded) == nil {
		if data, err := json.Marshal(decoded); err == nil {
			canonical = string(data)
		}
	}
	key := canonical + "\x00" + message
	t.attempts[key]++
	if t.attempts[key] > 3 {
		return "", &blockedError{message: "相同工具请求已多次失败，已暂停重复尝试。请调整请求或补充必要条件后继续。"}
	}
	if t.budget != nil && !t.budget.take() {
		return "", &blockedError{message: "本轮已多次尝试修正工具调用，仍无法完成。请缩小任务范围或补充必要条件后继续。"}
	}
	retry := t.attempts[key] < 3
	next := "根据错误修正参数或补充只读证据，再调用工具。验证失败不表示检查通过；不要重放修改操作。"
	if !retry {
		next = "相同请求已失败三次。不要重复调用；改用其他只读检查，或向用户说明缺少的条件。"
	}
	data, _ := json.Marshal(struct {
		IsError   bool   `json:"is_error"`
		Error     string `json:"error"`
		Retryable bool   `json:"retryable"`
		NextStep  string `json:"next_step"`
	}{true, message, retry, next})
	if t.record != nil {
		if err := t.record(t.name + "：" + message); err != nil {
			return "", err
		}
	}
	return string(data), nil
}

func decodeToolArguments[T any](_ context.Context, args string) (any, error) {
	value := new(T)
	if err := json.Unmarshal([]byte(args), value); err != nil {
		return nil, correctable("工具参数格式不正确：" + err.Error() + "。请按字段类型重新传入。")
	}
	return value, nil
}

// Shared across tools, so alternating invalid calls cannot evade the limit.
type correctionBudget struct {
	mu       sync.Mutex
	failures int
}

func (b *correctionBudget) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	return b.failures <= 12
}
