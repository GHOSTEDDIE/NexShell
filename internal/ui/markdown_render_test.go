package ui

import (
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestAssistantMarkdownRendering(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	b := newChatBlock(chatMessage{"assistant", "## 检查结果\n\n**正常**\n\n- CPU\n- 内存\n\n```sh\nfree -m\n```\n\n| 项目 | 值 |\n| --- | --- |\n| CPU | 12% |"})
	if len(b.body.Segments) < 5 {
		t.Fatalf("markdown rendered as plain text: %d segments", len(b.body.Segments))
	}
	b.setText("**已完成**")
	bold := false
	for _, s := range b.body.Segments {
		if v, ok := s.(*widget.TextSegment); ok && v.Style.TextStyle.Bold {
			bold = true
		}
	}
	if !bold {
		t.Fatal("stream update lost Markdown formatting")
	}
}
