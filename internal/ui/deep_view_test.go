package ui

import (
	"encoding/json"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"image/png"
	"os"
	"testing"
)

func TestDeepProgressBlocks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(NewTheme(false))
	sub, _ := json.Marshal(agent.SubtaskEvent{RunID: "test/child/1", Agent: "operations_worker", State: "completed", Message: "已读取服务配置并返回证据。"})
	memory, _ := json.Marshal(agent.MemoryEvent{Scope: "host:lab", Status: "processed", Message: "已整理本轮记忆"})
	events := []domain.Event{{Kind: "user", Text: "检查服务异常并修复。"}, {Kind: "plan_updated", Text: `[{"Content":"定位异常","Status":"completed"},{"Content":"验证恢复结果","Status":"in_progress"}]`}, {Kind: "subtask_state", Text: string(sub)}, {Kind: "memory_state", Text: string(memory)}}
	messages := []chatMessage{}
	for _, event := range events {
		messages = appendChatEvent(messages, event)
	}
	if len(messages) != 4 || messages[1].kind != "plan" || messages[2].kind != "subtask" || messages[3].kind != "memory" {
		t.Fatal(messages)
	}
	view := newConversationView()
	view.update("deep-test", messages)
	for _, block := range view.blocks {
		if accordion, ok := block.content.(*widget.Accordion); ok {
			accordion.OpenAll()
		}
	}
	win := app.NewWindow("DeepAgent 助手")
	defer win.Close()
	win.SetContent(view.scroll)
	win.Resize(fyne.NewSize(480, 640))
	win.Show()
	if p := os.Getenv("NEXSHELL_DEEP_SCREENSHOT"); p != "" {
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, win.Canvas().Capture()); e != nil {
			t.Fatal(e)
		}
	}
}
