package ui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"strings"
	"testing"
	"time"
)

func TestStreamingDeliveryLatency(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	task := domain.Task{ID: "stream", Status: "running"}
	if err = db.Put("tasks", task.ID, task); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := make(chan taskSelection, 1)
	updates := make(chan taskUpdate, 20)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchTaskUpdates(ctx, db, requests, func(_ taskSelection, u taskUpdate) { updates <- u })
	}()
	defer func() { cancel(); <-done }()
	requests <- taskSelection{task.ID, 1}
	<-updates // Initial selection finished; start the real persistent delta path.
	start := time.Now()
	if _, err = db.Event(task.ID, "assistant_delta", "第一段中文"); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-updates:
		if len(u.messages) != 1 || u.messages[0].text != "第一段中文" {
			t.Fatal("stream text mismatch", u)
		}
		t.Logf("persisted delta to view update: %s", time.Since(start))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("stream text was not delivered within 200 ms")
	}
}
func BenchmarkAssistantStreamingLongReply(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	prefix := strings.Repeat("### 检查结果\n\n已确认服务器状态正常，**下一步**检查日志和配置。\n\n", 120)
	v := newConversationView()
	window := app.NewWindow("stream benchmark")
	window.SetContent(v.scroll)
	window.Resize(fyne.NewSize(420, 720))
	window.Show()
	defer window.Close()
	v.update("task", []chatMessage{{"assistant", prefix}})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.update("task", []chatMessage{{"assistant", prefix + strings.Repeat("继续", i%50+1)}})
	}
}

func TestConversationFollowsStreamWithoutStealingScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	restoreTheme(app)
	v := newConversationView()
	w := app.NewWindow("stream")
	w.SetContent(v.scroll)
	w.Resize(fyne.NewSize(420, 320))
	w.Show()
	defer w.Close()
	text := strings.Repeat("检查服务器状态。\n\n", 40)
	v.update("task", []chatMessage{{"assistant", text}})
	if v.scroll.Offset.Y <= 0 {
		t.Fatal("new streamed text remains outside the visible viewport")
	}
	v.scroll.ScrollToTop()
	v.update("task", []chatMessage{{"assistant", text + "继续检查日志。\n\n"}})
	if v.scroll.Offset.Y != 0 {
		t.Fatal("stream stole the user's history scroll position")
	}
	v.scroll.ScrollToBottom()
	v.update("task", []chatMessage{{"assistant", text + strings.Repeat("新的检查结果。\n\n", 10)}})
	bottom := v.scroll.Content.MinSize().Height - v.scroll.Size().Height
	if v.scroll.Offset.Y < bottom-2 {
		t.Fatal("tail following did not resume", v.scroll.Offset.Y, bottom)
	}
}

func TestStreamingBurstExactTextAndTaskSwitch(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"A", "B"} {
		if err = db.Put("tasks", id, domain.Task{ID: id, Status: "running"}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := make(chan taskSelection, 1)
	updates := make(chan taskUpdate, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchTaskUpdates(ctx, db, requests, func(_ taskSelection, u taskUpdate) { updates <- u })
	}()
	defer func() { cancel(); <-done }()
	requests <- taskSelection{"A", 1}
	<-updates
	const fragment = "中文 e\u0301 🙂 **结果**\n"
	for i := 0; i < 256; i++ {
		if _, err = db.Event("A", "assistant_delta", fragment); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Put("tasks", "A", domain.Task{ID: "A", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	frames := 0
	for {
		select {
		case update := <-updates:
			frames++
			if update.status != "completed" {
				continue
			}
			if len(update.messages) != 1 || update.messages[0].text != strings.Repeat(fragment, 256) {
				t.Fatal("coalescing changed unicode or lost text")
			}
			if frames >= 256 {
				t.Fatal("one render per token was queued")
			}
			t.Logf("256 committed fragments delivered in %d updates", frames)
		case <-time.After(2 * time.Second):
			t.Fatal("completion not delivered")
		}
		break
	}
	requests <- taskSelection{"B", 2}
	if u := <-updates; u.id != "B" {
		t.Fatal("wrong selection", u.id)
	}
	db.Event("A", "assistant_delta", "旧任务继续输出")
	db.Event("B", "assistant_delta", "当前任务")
	select {
	case u := <-updates:
		if u.id != "B" || len(u.messages) != 1 || u.messages[0].text != "当前任务" {
			t.Fatal("stream crossed tasks", u)
		}
	case <-time.After(time.Second):
		t.Fatal("new selection lost subscription")
	}
}
