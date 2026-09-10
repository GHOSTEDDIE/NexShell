package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// blockedError is a known limit requiring a different request or configuration,
// not a successful operation and not an unhandled application failure.
type blockedError struct {
	message string
	cause   error
}

func (e *blockedError) Error() string { return e.message }
func (e *blockedError) Unwrap() error { return e.cause }

type resilientModel struct{ model.ToolCallingChatModel }

func (m *resilientModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.ToolCallingChatModel.WithTools(infos)
	if err != nil {
		return nil, err
	}
	return &resilientModel{bound}, nil
}
func (m *resilientModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return retryModelCall(ctx, func() (*schema.Message, error) { return m.ToolCallingChatModel.Generate(ctx, msgs, opts...) })
}
func (m *resilientModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// Retry only stream establishment. Once a reader is returned, never replay a
	// stream that could already have produced text or tool calls.
	return retryModelCall(ctx, func() (*schema.StreamReader[*schema.Message], error) {
		return m.ToolCallingChatModel.Stream(ctx, msgs, opts...)
	})
}
func retryModelCall[T any](ctx context.Context, call func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		out, err := call()
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		retry, message := modelErrorAction(err)
		if !retry || attempt == 2 {
			if message != "" {
				return zero, &blockedError{message: message, cause: err}
			}
			return zero, err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, fmt.Errorf("模型请求未完成")
}
func modelErrorAction(err error) (bool, string) {
	var api *openai.APIError
	if errors.As(err, &api) {
		switch api.HTTPStatusCode {
		case 401, 403:
			return false, "模型服务未通过身份验证，请在设置中检查 API 密钥及访问权限。"
		case 400, 404, 422:
			return false, "模型服务不支持当前请求，请检查模型名称、接口地址和工具调用能力。"
		case 429, 500, 502, 503, 504:
			return true, "模型服务暂时不可用，已达到三次请求上限。请稍后继续，或更换可用模型。"
		}
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return true, "模型连接超时，已达到三次请求上限。请检查网络和模型服务后继续。"
	}
	return false, ""
}
