package store

import (
	"context"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"testing"
	"time"
)

func TestExecutionReplayAndRecovery(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	req := domain.Request{TaskID: "task", CallID: "call", HostID: "host", Operation: "shell", Command: "echo ok"}
	r, fresh, e := s.Claim(req)
	if e != nil || !fresh {
		t.Fatalf("claim: %v %v", fresh, e)
	}
	same, fresh, e := s.Claim(req)
	if e != nil || fresh || same.ID != r.ID {
		t.Fatalf("duplicate dispatched: %v %v", fresh, e)
	}
	changed := req
	changed.Command = "echo changed"
	if _, _, e = s.Claim(changed); e == nil {
		t.Fatal("changed call accepted")
	}
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	results, _ := s.Results("task")
	if results[0].Status != "unknown" {
		t.Fatalf("unexpected recovery: %+v", results)
	}
	_, fresh, e = s.Claim(req)
	if e != nil || fresh {
		t.Fatal("unknown execution replayed")
	}
}
func TestCheckpointAndEvents(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	cp := Checkpoints{s}
	if e = cp.Set(ctx, "task", []byte("checkpoint")); e != nil {
		t.Fatal(e)
	}
	b, ok, e := cp.Get(ctx, "task")
	if e != nil || !ok || string(b) != "checkpoint" {
		t.Fatal("checkpoint not durable")
	}
	if e = cp.Delete(ctx, "task"); e != nil {
		t.Fatal(e)
	}
	_, ok, _ = cp.Get(ctx, "task")
	if ok {
		t.Fatal("checkpoint not deleted")
	}
	a, _ := s.Event("task", "start", "one")
	_, _ = s.Event("other", "start", "other")
	bEvent, _ := s.Event("task", "done", "two")
	events, e := s.Events("task", a.Sequence)
	if e != nil || len(events) != 1 || events[0].Sequence != bEvent.Sequence {
		t.Fatal("event cursor isolation failed")
	}
	task := domain.Task{ID: "task", Status: "running", Grant: domain.Grant{ExpiresAt: time.Now().Add(time.Hour)}}
	s.Put("tasks", task.ID, task)
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	s.Load("tasks", task.ID, &task)
	if task.Status != "interrupted" || !task.Grant.ExpiresAt.IsZero() {
		t.Fatal("recovery retained live authority")
	}
}
