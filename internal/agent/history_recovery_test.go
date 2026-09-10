package agent

import (
	"github.com/cloudwego/eino/schema"
	"reflect"
	"strings"
	"testing"
)

func TestInterruptedToolHistoryPreservesCompletedResults(t *testing.T) {
	call := schema.AssistantMessage("检查", []schema.ToolCall{{ID: "done"}, {ID: "missing"}})
	done := &schema.Message{Role: schema.Tool, ToolCallID: "done", Content: "真实证据"}
	user := schema.UserMessage("继续")
	got := completeInterruptedToolCalls([]*schema.Message{call, done, user})
	if len(got) != 4 || got[0] != call || got[1] != done || got[3] != user || got[2].ToolCallID != "missing" || !strings.Contains(got[2].Content, `"is_error":true`) {
		t.Fatalf("incorrect history repair: %+v", got)
	}
	if again := completeInterruptedToolCalls(got); !reflect.DeepEqual(got, again) {
		t.Fatal("history repair not idempotent")
	}
	normal := []*schema.Message{schema.UserMessage("你好"), schema.AssistantMessage("回答", nil)}
	if !reflect.DeepEqual(normal, completeInterruptedToolCalls(normal)) {
		t.Fatal("valid history changed")
	}
}
