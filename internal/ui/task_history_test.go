package ui

import (
	"reflect"
	"testing"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
)

type historySource struct {
	*store.Store
	offsets []int64
}

func (s *historySource) Events(id string, after int64) ([]domain.Event, error) {
	s.offsets = append(s.offsets, after)
	return s.Store.Events(id, after)
}
func TestTaskHistoryIncrementalAndStateChanges(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := &historySource{Store: db}
	task := domain.Task{ID: "task-a", Status: "running"}
	save := func() {
		t.Helper()
		if err := db.Put("tasks", task.ID, task); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent := func(kind, text string) int64 {
		t.Helper()
		e, err := db.Event(task.ID, kind, text)
		if err != nil {
			t.Fatal(err)
		}
		return e.Sequence
	}
	save()
	appendEvent("user", "检查")
	appendEvent("assistant_delta", "好的")
	last := appendEvent("execution_delta", "进行中")
	var h taskHistory
	got, changed, err := h.load(source, task.ID)
	if err != nil || !changed || got.text != "\n\n你：检查\n\n好的进行中" {
		t.Fatalf("initial: %+v %t %v", got, changed, err)
	}
	if _, changed, err = h.load(source, task.ID); err != nil || changed {
		t.Fatalf("unchanged history repainted: %t %v", changed, err)
	}
	final := appendEvent("execution_result", "结果")
	got, _, err = h.load(source, task.ID)
	if err != nil || got.text != "\n\n你：检查\n\n好的进行中\n结果\n" {
		t.Fatalf("increment: %+v %v", got, err)
	}
	task.Status = "completed"
	save()
	got, changed, err = h.load(source, task.ID)
	if err != nil || !changed || got.text != "\n\n你：检查\n\n好的\n结果\n" {
		t.Fatalf("completed: %+v %t %v", got, changed, err)
	}
	task.Status = "failed"
	task.Summary = "连接中断"
	save()
	got, _, _ = h.load(source, task.ID)
	if got.text != "\n\n你：检查\n\n好的\n结果\n\n连接中断" {
		t.Fatal(got.text)
	}
	task.Status = "running"
	save()
	got, _, _ = h.load(source, task.ID)
	if got.text != "\n\n你：检查\n\n好的进行中\n结果\n" {
		t.Fatal("resume lost running presentation", got.text)
	}
	if !reflect.DeepEqual(source.offsets, []int64{0, last, last, final, final, final}) {
		t.Fatal("old events reread", source.offsets)
	}
	// Switching away releases the old cache; switching back reloads the right task.
	other := domain.Task{ID: "task-b", Status: "ready"}
	if err := db.Put("tasks", other.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Event(other.ID, "assistant", "另一个任务"); err != nil {
		t.Fatal(err)
	}
	got, _, _ = h.load(source, other.ID)
	if got.text != "另一个任务" {
		t.Fatal(got.text)
	}
	got, _, _ = h.load(source, task.ID)
	if got.id != task.ID || got.text != "\n\n你：检查\n\n好的进行中\n结果\n" {
		t.Fatal("task cache crossed", got)
	}
}
func TestTaskSelectionCoalescesWithoutDatabaseWork(t *testing.T) {
	u := &App{taskRequests: make(chan taskSelection, 1)}
	for _, id := range []string{"A", "B", "A"} {
		u.taskID = id
		u.updateTask()
	}
	got := <-u.taskRequests
	if got.id != "A" || got.revision != 3 || len(u.taskRequests) != 0 {
		t.Fatal(got)
	}
}

func TestBlockedTaskShowsActionableNotice(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	task := domain.Task{ID: "blocked", Status: "blocked", Summary: "模型服务未通过身份验证，请检查 API 密钥。"}
	if err = db.Put("tasks", task.ID, task); err != nil {
		t.Fatal(err)
	}
	var h taskHistory
	got, changed, err := h.load(db, task.ID)
	if err != nil || !changed || got.status != "blocked" || len(got.messages) != 1 || got.messages[0].kind != "notice" || got.messages[0].text != task.Summary {
		t.Fatalf("blocked reason not visible: %+v %v", got, err)
	}
	task.Status = "running"
	if err = db.Put("tasks", task.ID, task); err != nil {
		t.Fatal(err)
	}
	got, _, err = h.load(db, task.ID)
	if err != nil || len(got.messages) != 0 {
		t.Fatal("stale blocked notice remained after continuing")
	}
}
