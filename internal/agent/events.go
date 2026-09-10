package agent

import (
	"context"
	"errors"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"io"
	"strings"
)

type eventState struct {
	pending bool
	err     error
	summary string
}

func (s *Service) consumeEvents(ctx context.Context, id string, state *eventState, events *adk.AsyncIterator[*adk.AgentEvent], appendHistory func(*schema.Message) error, stop func()) error {

	hasAssistantOutput := false
	for {
		ev, ok := events.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			var cancelled *adk.CancelError
			if errors.As(ev.Err, &cancelled) {
				continue
			}
			state.err = ev.Err
			return ev.Err
		}
		if ev.Action != nil && ev.Action.Interrupted != nil {
			state.pending = true
			for _, point := range ev.Action.Interrupted.InterruptContexts {
				if a, ok := point.Info.(*Approval); ok {
					a.InterruptID = point.ID
					if e := s.Store.Put("approvals", a.Digest, a); e != nil {
						return e
					}
					if e := s.event(id, "approval_required", a.Digest); e != nil {
						return e
					}
				}
			}
		}
		if ev.AgentName != "" && ev.AgentName != "operator" {
			if ev.Output != nil && ev.Output.MessageOutput != nil {
				if _, err := ev.Output.MessageOutput.GetMessage(); err != nil {
					return err
				}
			}
			continue
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			mv := ev.Output.MessageOutput
			var msg *schema.Message
			if mv.IsStreaming {
				stream := mv.MessageStream
				role := mv.Role
				var chunks []*schema.Message
				for {
					chunk, e := stream.Recv()
					if e == io.EOF {
						break
					}
					if e != nil {
						stream.Close()
						state.err = e
						if role == schema.Assistant && ctx.Err() == nil {
							state.err = &blockedError{message: "模型回复在传输中断开，本轮没有自动重放。请检查模型连接后继续提问。", cause: e}
						}
						return state.err
					}
					chunks = append(chunks, chunk)
					if chunk.Role != "" {
						role = chunk.Role
					}
					if role == schema.Assistant && chunk.Content != "" {
						if e = s.event(id, "assistant_delta", chunk.Content); e != nil {
							stream.Close()
							return e
						}
					}
				}
				stream.Close()
				var e error
				msg, e = schema.ConcatMessages(chunks)
				if e != nil {
					return e
				}
			} else {
				msg = mv.Message
			}
			if msg != nil {
				if msg.Role == schema.Assistant {
					if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
						state.err = &blockedError{message: "模型未返回有效回复，请重试或在设置中更换支持工具调用的模型。"}
						return state.err
					}
					hasAssistantOutput = true
				}
				if e := appendHistory(msg); e != nil {
					return e
				}
				if msg.Role == schema.Assistant && msg.Content != "" {
					state.summary = msg.Content
					if !mv.IsStreaming {
						if e := s.event(id, "assistant", msg.Content); e != nil {
							return e
						}
					}
				}
			}
		}
	}
	if !state.pending && !hasAssistantOutput {
		state.err = &blockedError{message: "模型未返回有效回复，请重试或在设置中更换支持工具调用的模型。"}
		return state.err
	}
	if !state.pending {
		stop()
	}
	return nil
}
