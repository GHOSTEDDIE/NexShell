package agent

import "github.com/cloudwego/eino/schema"

// A failed tool node may have persisted its assistant call but no tool response.
// Complete only those missing responses so continuing a failed conversation
// satisfies the model protocol without pretending an operation succeeded.
func completeInterruptedToolCalls(history []*schema.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(history))
	for i := 0; i < len(history); i++ {
		msg := history[i]
		out = append(out, msg)
		if msg.Role != schema.Assistant || len(msg.ToolCalls) == 0 {
			continue
		}
		answered := map[string]bool{}
		for i+1 < len(history) && history[i+1].Role == schema.Tool {
			i++
			out = append(out, history[i])
			answered[history[i].ToolCallID] = true
		}
		for _, call := range msg.ToolCalls {
			if !answered[call.ID] {
				out = append(out, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: `{"is_error":true,"retryable":false,"error":"上轮调用已中断，未获得工具返回，执行结果不能确认。","next_step":"先用只读检查核实状态，禁止直接重放修改操作。"}`})
			}
		}
	}
	return out
}
