package agent

import (
	"context"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/cloudwego/eino/schema"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestModelTransientHTTPFailure(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", stream), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(503)
					fmt.Fprint(w, `{"error":{"message":"temporarily unavailable","type":"server_error"}}`)
					return
				}
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"id\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
				}
			}))
			defer server.Close()
			cm, e := NewModel(context.Background(), domain.ModelProfile{ID: "test", Provider: "openai", BaseURL: server.URL + "/v1", Model: "fixture"}, testSecrets{})
			if e != nil {
				t.Fatal(e)
			}
			if stream {
				r, e := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
				if e != nil {
					t.Fatal(e)
				}
				defer r.Close()
				content := ""
				for {
					m, e := r.Recv()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal(e)
					}
					content += m.Content
				}
				if content != "ok" {
					t.Fatal(content)
				}
			} else {
				m, e := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
				if e != nil || m.Content != "ok" {
					t.Fatal(m, e)
				}
			}
			if calls.Load() != 2 {
				t.Fatalf("unexpected attempts: %d", calls.Load())
			}
		})
	}
}

func TestModelPermanentAndExhaustedFailures(t *testing.T) {
	for _, tc := range []struct{ code, wantCalls int }{{401, 1}, {403, 1}, {400, 1}, {404, 1}, {429, 3}, {503, 3}} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, `{"error":{"message":"fixture failure","type":"api_error"}}`)
			}))
			defer server.Close()
			cm, e := NewModel(context.Background(), domain.ModelProfile{ID: "test", Provider: "openai", BaseURL: server.URL + "/v1", Model: "fixture"}, testSecrets{})
			if e != nil {
				t.Fatal(e)
			}
			task, s, _ := runBoundaryConversation(t, cm)
			if task.Status != "blocked" || calls.Load() != int32(tc.wantCalls) {
				t.Fatalf("status=%s calls=%d summary=%s", task.Status, calls.Load(), task.Summary)
			}
			if results, _ := s.Results(task.ID); len(results) > 0 {
				t.Fatal("model failure executed tools")
			}
		})
	}
}

func TestModelPartialStreamIsNeverRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", "10000")
		fmt.Fprint(w, "data: {\"id\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"partial\"}}]}\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	cm, e := NewModel(context.Background(), domain.ModelProfile{ID: "test", Provider: "openai", BaseURL: server.URL + "/v1", Model: "fixture"}, testSecrets{})
	if e != nil {
		t.Fatal(e)
	}
	reader, e := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	var content string
	for i := 0; i < 10; i++ {
		msg, err := reader.Recv()
		if err != nil {
			if calls.Load() != 1 {
				t.Fatal("partial stream replayed")
			}
			if content != "partial" {
				t.Fatalf("partial response lost: %s", content)
			}
			return
		}
		content += msg.Content
	}
	t.Fatal("partial stream did not terminate")
}

func TestModelRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"busy"}}`)
		cancel()
	}))
	defer server.Close()
	cm, e := NewModel(ctx, domain.ModelProfile{ID: "test", Provider: "openai", BaseURL: server.URL + "/v1", Model: "fixture"}, testSecrets{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = cm.Stream(ctx, []*schema.Message{schema.UserMessage("hi")})
	if !errors.Is(e, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("cancellation ignored: %v calls=%d", e, calls.Load())
	}
}

func TestInvalidModelResponsesDoNotSucceed(t *testing.T) {
	for _, response := range []string{"data: [DONE]\n\n", "data: {bad-json}\n\n", "data: {\"id\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"bad-call\",\"type\":\"function\",\"function\":{\"name\":\"unknown\",\"arguments\":\"{\"}}]}}]}\n\ndata: [DONE]\n\n"} {
		t.Run(fmt.Sprint(len(response)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			cm, e := NewModel(context.Background(), domain.ModelProfile{ID: "test", Provider: "openai", BaseURL: server.URL + "/v1", Model: "fixture"}, testSecrets{})
			if e != nil {
				t.Fatal(e)
			}
			task, s, _ := runBoundaryConversation(t, cm)
			if task.Status == "ready" || task.Status == "completed" || task.Status == "running" {
				t.Fatalf("invalid response treated as success: %s %s", task.Status, task.Summary)
			}
			if results, _ := s.Results(task.ID); len(results) != 0 {
				t.Fatal("invalid response caused execution")
			}
		})
	}
}
