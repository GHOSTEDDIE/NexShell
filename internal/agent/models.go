package agent

import (
	"context"
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"net/url"
	"time"
)

func NewModel(ctx context.Context, p domain.ModelProfile, secrets remote.Secrets) (model.ToolCallingChatModel, error) {
	u, err := url.Parse(p.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("模型服务地址必须是有效的 HTTP 或 HTTPS 地址")
	}
	if p.Model == "" {
		return nil, errors.New("请填写模型名称")
	}
	switch p.Provider {
	case "openai":
		key, err := secrets.Get("model:" + p.ID)
		if err != nil {
			return nil, err
		}
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey: key, BaseURL: p.BaseURL, Model: p.Model, Timeout: 180 * time.Second})
	case "ollama":
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{BaseURL: p.BaseURL, Model: p.Model, Timeout: 180 * time.Second})
	}
	return nil, errors.New("不支持的模型接口")
}
