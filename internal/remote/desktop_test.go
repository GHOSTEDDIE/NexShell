package remote

import (
	"bytes"
	"context"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"testing"
)

func TestDesktopTerminalTargetAndReplay(t *testing.T) {
	d := &DesktopTerminals{}
	var a, b bytes.Buffer
	idA := d.Register("A", &a, func() string { return "中文 baseline" }, func() bool { return true })
	idB := d.Register("B", &b, func() string { return "other" }, func() bool { return true })
	d.Select(idB)
	list := d.List([]string{"A"})
	if len(list) != 1 || list[0].ID != idA || list[0].Active {
		t.Fatal("host scope leaked")
	}
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := &Executor{Store: s, Desktop: d}
	r := domain.Request{TaskID: "task", CallID: "call", HostID: "A", Operation: "terminal_write", Resource: idA, Content: "pwd\n"}
	first, err := e.Execute(context.Background(), r)
	if err != nil || first.Status != "succeeded" {
		t.Fatal(first, err)
	}
	again, err := e.Execute(context.Background(), r)
	if err != nil || again.ID != first.ID || a.String() != "pwd\n" || b.Len() != 0 {
		t.Fatal("write replayed or selection retargeted")
	}
	r.HostID = "B"
	if _, err := d.Perform(context.Background(), r); err == nil {
		t.Fatal("cross-host write accepted")
	}
	r.HostID = "A"
	r.Operation = "terminal_read"
	out, err := d.Perform(context.Background(), r)
	if err != nil || out != "中文 baseline" {
		t.Fatal(out, err)
	}
	d.Remove(idA)
	if _, err := d.Perform(context.Background(), r); err == nil {
		t.Fatal("closed terminal reused")
	}
}
