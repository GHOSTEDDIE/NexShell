package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"testing"
)

// Exercise the actual composer's send callback, including first-conversation
// setup, without contacting any model provider or server.
func TestAssistantFirstSendWithDefaultProfile(t *testing.T) {
	for _, id := range []string{"default", "12345678-1234-4567-89ab-123456789abc"} {
		for _, method := range []string{"enter", "button"} {
			t.Run(id+"/"+method, func(t *testing.T) { testAssistantFirstSend(t, id, method) })
		}
	}
}

func testAssistantFirstSend(t *testing.T, profileID, method string) {
	app := test.NewApp()
	defer app.Quit()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	profile := domain.ModelProfile{ID: profileID, Name: "默认模型", Provider: "openai", Model: "test-model", BaseURL: "http://127.0.0.1", ContextTokens: 32000}
	if err = s.Put("models", profile.ID, profile); err != nil {
		t.Fatal(err)
	}
	manager, err := remote.NewManager(s, store.Credentials{}, s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ex := &remote.Executor{Manager: manager, Store: s}
	agents := agent.NewService(s, ex, store.Credentials{})
	defer agents.Close()
	u := New(app, s, manager, ex, agents)
	defer func() { u.cancel(); u.Window.SetCloseIntercept(nil); u.Window.Close() }()
	u.hosts = []domain.Host{{ID: "local", Name: "测试服务器", User: "test", Address: "127.0.0.1"}}
	names, ids := u.hostChoices()
	if len(names) != 1 || ids[names[0]] != "local" {
		t.Fatal("host selector changed the stored identifier")
	}
	u.Window.Show()
	input := findComposer(u.desktopBody)
	if input == nil {
		t.Fatal("composer not found")
	}
	input.SetText("你好")
	if method == "enter" {
		input.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	} else {
		send := findSendButton(u.desktopBody)
		if send == nil {
			t.Fatal("send button not found")
		}
		test.Tap(send)
	}
	if len(u.Window.Canvas().Overlays().List()) == 0 {
		t.Fatal("first send did not open conversation setup")
	}
	if input.Text != "你好" {
		t.Fatal("draft cleared before conversation confirmation")
	}
}
func findComposer(o fyne.CanvasObject) *chatInput {
	switch v := o.(type) {
	case *chatInput:
		return v
	case *fyne.Container:
		for _, child := range v.Objects {
			if found := findComposer(child); found != nil {
				return found
			}
		}
	case *surface:
		return findComposer(v.Content)
	case *container.ThemeOverride:
		return findComposer(v.Content)
	}
	return nil
}

func findSendButton(o fyne.CanvasObject) *actionButton {
	switch v := o.(type) {
	case *actionButton:
		if v.Icon != nil && v.Icon.Name() == designIcon("send").Name() {
			return v
		}
	case *fyne.Container:
		for _, child := range v.Objects {
			if found := findSendButton(child); found != nil {
				return found
			}
		}
	case *surface:
		return findSendButton(v.Content)
	case *container.ThemeOverride:
		return findSendButton(v.Content)
	}
	return nil
}

func TestDisplayIdentifier(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"", ""}, {"default", "default"}, {"12345678", "12345678"},
		{"12345678-1234-4567-89ab-123456789abc", "12345678"},
		{"本地默认模型配置一", "本地默认模型配置"},
	} {
		if got := shortID(tc.id); got != tc.want {
			t.Errorf("shortID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}
