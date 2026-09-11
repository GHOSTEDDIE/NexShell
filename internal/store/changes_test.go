package store

import (
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"sync"
	"testing"
	"time"
)

func TestTaskChangesCommitCoalesceAndClose(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ch, stop := s.SubscribeTask("A")
	other, stopOther := s.SubscribeTask("B")
	defer stopOther()
	for i := 0; i < 100; i++ {
		if _, err = s.Event("A", "assistant_delta", "字"); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("missing task wake-up")
	}
	if len(ch) != 0 {
		t.Fatal("notifications accumulated")
	}
	events, err := s.Events("A", 0)
	if err != nil || len(events) != 100 {
		t.Fatal("coalescing lost committed text", len(events), err)
	}
	select {
	case <-other:
		t.Fatal("task notification crossed scope")
	default:
	}
	if err = s.Put("tasks", "A", domain.Task{ID: "A", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	<-ch
	var task domain.Task
	if err = s.Load("tasks", "A", &task); err != nil || task.Status != "completed" {
		t.Fatal("notification preceded commit", err)
	}
	stop()
	stop()
	if _, ok := <-ch; ok {
		t.Fatal("subscription not closed")
	}
	s.Close()
	if _, ok := <-other; ok {
		t.Fatal("store close leaked subscription")
	}
	late, _ := s.SubscribeTask("A")
	if _, ok := <-late; ok {
		t.Fatal("closed store accepted subscriber")
	}
}
func TestTaskChangesConcurrentRelease(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, stop := s.SubscribeTask("task")
				s.changes.notify("task")
				stop()
			}
		}()
	}
	wg.Wait()
	s.changes.mu.Lock()
	defer s.changes.mu.Unlock()
	if len(s.changes.subscribers) != 0 {
		t.Fatal("subscription leaked")
	}
}
