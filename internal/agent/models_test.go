package agent

import (
	"context"
	"encoding/json"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testSecrets struct{}

func (testSecrets) Get(string) (string, error) { return "nonsecret-test-token", nil }
func TestModelAdaptersAndToolBinding(t *testing.T) {
	for _, provider := range []string{"openai", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				var body map[string]any
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					t.Error(e)
				}
				if body["model"] != "fixture" {
					t.Error("model not forwarded")
				}
				if tools, ok := body["tools"].([]any); !ok || len(tools) != 1 {
					t.Error("tools not forwarded")
				}
				w.Header().Set("Content-Type", "application/json")
				if provider == "openai" {
					if r.URL.Path != "/v1/chat/completions" {
						t.Error(r.URL.Path)
					}
					w.Write([]byte(`{"id":"test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
				} else {
					if r.URL.Path != "/api/chat" {
						t.Error(r.URL.Path)
					}
					w.Write([]byte(`{"model":"fixture","message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop"}`))
				}
			}))
			defer server.Close()
			base := server.URL
			if provider == "openai" {
				base += "/v1"
			}
			cm, e := NewModel(context.Background(), domain.ModelProfile{ID: "test", Provider: provider, BaseURL: base, Model: "fixture"}, testSecrets{})
			if e != nil {
				t.Fatal(e)
			}
			cm, e = cm.WithTools([]*schema.ToolInfo{{Name: "observe", Desc: "observe the selected host"}})
			if e != nil {
				t.Fatal(e)
			}
			msg, e := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
			if e != nil || msg.Content != "ok" || !called {
				t.Fatal(msg, e)
			}
		})
	}
}
