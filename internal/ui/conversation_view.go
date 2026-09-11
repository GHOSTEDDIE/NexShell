package ui

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"strings"
)

type chatMessage struct{ kind, text string }

func appendChatEvent(messages []chatMessage, e domain.Event) []chatMessage {
	kind := e.Kind
	switch kind {
	case "user":
		// The routing context appended by conversationInput is operational metadata,
		// not part of the user's visible message.
		e.Text = strings.Split(e.Text, "\n\n[发送时")[0]
	case "assistant_delta", "assistant":
		kind = "assistant"
	case "execution_start", "execution_result", "tool_feedback":
		kind = "execution"
	case "verified":
		kind = "verified"
	case "plan_updated":
		var todos []struct{ Content, Status string }
		if json.Unmarshal([]byte(e.Text), &todos) != nil {
			return messages
		}
		var lines []string
		for _, todo := range todos {
			mark := "○"
			if todo.Status == "in_progress" {
				mark = "▶"
			} else if todo.Status == "completed" {
				mark = "✓"
			}
			lines = append(lines, mark+" "+todo.Content)
		}
		e.Text = strings.Join(lines, "\n")
		kind = "plan"
	case "subtask_state":
		var sub agent.SubtaskEvent
		if json.Unmarshal([]byte(e.Text), &sub) != nil {
			return messages
		}
		state := map[string]string{"running": "执行中", "completed": "已返回结果", "interrupted": "等待恢复", "failed": "未完成"}[sub.State]
		e.Text = fmt.Sprintf("%s · %s\n%s", agentLabel(sub.Agent), state, sub.Message)
		kind = "subtask"
	case "memory_state":
		var memory agent.MemoryEvent
		if json.Unmarshal([]byte(e.Text), &memory) == nil {
			e.Text = memory.Message
		}
		kind = "memory"
	case "approval_required":
		e.Text = "需要确认具体操作，请点击“待确认操作”。"
		kind = "notice"
	default:
		return messages
	}
	if n := len(messages); n > 0 && messages[n-1].kind == kind && (kind == "assistant" || kind == "execution") {
		sep := ""
		if kind == "execution" {
			sep = "\n"
		}
		messages[n-1].text += sep + e.Text
		return messages
	}
	return append(messages, chatMessage{kind, e.Text})
}

type chatBlock struct {
	kind, last string
	body       *widget.RichText
	content    fyne.CanvasObject
}
type conversationView struct {
	content *fyne.Container
	scroll  *container.Scroll
	id      string
	blocks  []*chatBlock
}

func newConversationView() *conversationView {
	v := &conversationView{content: container.NewVBox()}
	v.scroll = container.NewVScroll(inset(v.content, 16, 18, 16, 18))
	v.empty()
	return v
}
func (v *conversationView) empty() {
	v.content.Objects = []fyne.CanvasObject{gap(24), headingText("有什么可以帮你？"), metaText("询问问题，或检查当前服务器的运行状态。")}
	v.content.Refresh()
}
func (v *conversationView) reset(id string) {
	v.id = id
	v.blocks = nil
	v.empty()
	v.scroll.Offset = fyne.Position{}
}
func (v *conversationView) update(id string, messages []chatMessage) {
	follow := v.id != id || len(v.blocks) == 0 || v.scroll.Offset.Y >= max(float32(0), v.scroll.Content.MinSize().Height-v.scroll.Size().Height)-24
	if v.id != id {
		v.reset(id)
	}
	if len(messages) == 0 {
		return
	}
	if len(v.blocks) == 0 {
		v.content.Objects = nil
	}
	for i, m := range messages {
		if i >= len(v.blocks) {
			block := newChatBlock(m)
			v.blocks = append(v.blocks, block)
			v.content.Add(block.content)
			v.content.Add(gap(16))
		} else if v.blocks[i].last != m.text {
			v.blocks[i].setText(m.text)
		}
	}
	v.scroll.Refresh()
	if follow {
		v.scroll.ScrollToBottom()
	}
}
func newChatBlock(m chatMessage) *chatBlock {
	b := &chatBlock{kind: m.kind, body: widget.NewRichText()}
	b.body.Wrapping = fyne.TextWrapWord
	b.setText(m.text)
	switch m.kind {
	case "user":
		b.content = inset(panel(padded(b.body, 12), colorMessage, false, 7), 0, 0, 0, 20)
	case "plan", "subtask", "memory":
		titles := map[string]string{"plan": "任务计划", "subtask": "子任务", "memory": "记忆状态"}
		b.content = widget.NewAccordion(widget.NewAccordionItem(titles[m.kind], boundedChatContent(b.body, 120)))
	case "execution":
		b.content = widget.NewAccordion(widget.NewAccordionItem("执行记录", boundedChatContent(b.body, 180)))
	case "verified":
		b.content = panel(padded(container.NewVBox(textUI("✓ 检查完成", sizeControl, theme.ColorNameSuccess, true), widget.NewSeparator(), b.body), 14), theme.ColorNameBackground, true, 7)
	case "notice":
		b.content = panel(padded(b.body, 12), colorMessage, true, 6)
	default:
		b.content = container.NewVBox(container.NewHBox(widget.NewIcon(designIcon("spark")), textUI("NexShell", sizeControl, theme.ColorNameForeground, true)), gap(8), b.body)
	}
	return b
}
func (b *chatBlock) setText(s string) {
	b.last = s
	if b.kind == "assistant" {
		b.body.ParseMarkdown(s)
		return
	}
	b.body.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: s, Style: widget.RichTextStyle{Inline: true, SizeName: sizeBody, ColorName: theme.ColorNameForeground, TextStyle: fyne.TextStyle{Monospace: b.kind == "execution"}}}}
	b.body.Refresh()
}

type chatInput struct {
	widget.Entry
	Submit       func()
	PasteFiles   func() ([]string, error)
	OnFiles      func([]string)
	OnPasteError func(error)
}

func newChatInput() *chatInput {
	v := &chatInput{}
	v.MultiLine = true
	v.Wrapping = fyne.TextWrapWord
	v.ExtendBaseWidget(v)
	v.SetMinRowsVisible(2)
	return v
}
func (v *chatInput) TypedShortcut(shortcut fyne.Shortcut) {
	if _, ok := shortcut.(*fyne.ShortcutPaste); ok && !v.Disabled() && v.PasteFiles != nil && v.OnFiles != nil {
		paths, err := v.PasteFiles()
		if err != nil {
			if v.OnPasteError != nil {
				v.OnPasteError(err)
			}
			return
		}
		if len(paths) > 0 {
			v.OnFiles(paths)
			return
		}
	}
	v.Entry.TypedShortcut(shortcut)
}
func (v *chatInput) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		shift := false
		if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			shift = d.CurrentKeyModifiers()&fyne.KeyModifierShift != 0
		}
		if !shift {
			if v.Submit != nil {
				v.Submit()
			}
			return
		}
	}
	v.Entry.TypedKey(e)
}

func agentLabel(name string) string {
	switch name {
	case "operator":
		return "主助手"
	case "operations_worker":
		return "运维子助手"
	}
	return "助手"
}

func boundedChatContent(body fyne.CanvasObject, height float32) fyne.CanvasObject {
	scroll := container.NewVScroll(body)
	scroll.SetMinSize(fyne.NewSize(0, height))
	return scroll
}
