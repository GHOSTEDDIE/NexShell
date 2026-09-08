package agent

import (
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"testing"
	"time"
)

func TestGrantIsolation(t *testing.T) {
	h := domain.Host{ID: "A", Address: "host", User: "ops"}
	task := domain.Task{ID: "task", Grant: domain.Grant{ID: "g", HostIDs: []string{"A"}, Identities: map[string]string{"A": domain.Digest(h)}, Operations: []string{"observe", "file_write", "shell"}, Resources: []string{"/etc/service.conf"}, ExpiresAt: time.Now().Add(time.Hour)}}
	req := domain.Request{TaskID: task.ID, HostID: h.ID, Operation: "observe"}
	if allowed, e := Evaluate(task, h, req, time.Now()); e != nil || !allowed {
		t.Fatal(allowed, e)
	}
	tests := []struct {
		name    string
		req     domain.Request
		allowed bool
		denied  bool
	}{{"host change", domain.Request{TaskID: "task", HostID: "B", Operation: "observe"}, false, true}, {"path escape", domain.Request{TaskID: "task", HostID: "A", Operation: "file_write", Resource: "/etc/../root/key"}, false, true}, {"exact file", domain.Request{TaskID: "task", HostID: "A", Operation: "file_write", Resource: "/etc/service.conf"}, true, false}, {"different file", domain.Request{TaskID: "task", HostID: "A", Operation: "file_write", Resource: "/etc/other"}, false, false}, {"shell never inherits", domain.Request{TaskID: "task", HostID: "A", Operation: "shell", Command: "echo anything"}, false, false}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			allowed, e := Evaluate(task, h, tc.req, time.Now())
			if allowed != tc.allowed || (e != nil) != tc.denied {
				t.Fatalf("allowed=%v err=%v", allowed, e)
			}
		})
	}
	h.User = "root"
	if _, e := Evaluate(task, h, req, time.Now()); e == nil {
		t.Fatal("identity drift accepted")
	}
}
func TestApprovalBoundToExactRequest(t *testing.T) {
	task := domain.Task{ID: "task", Grant: domain.Grant{ID: "grant"}}
	r := domain.Request{TaskID: "task", HostID: "A", CallID: "1", Operation: "shell", Command: "echo approved"}
	a := Approval{Request: r, Digest: domain.Digest(r), GrantID: "grant", Approved: true, Decided: true}
	if !ApprovedFor(a, task, r) {
		t.Fatal("exact approval rejected")
	}
	r.Command = "echo changed"
	if ApprovedFor(a, task, r) {
		t.Fatal("changed command accepted")
	}
	r = a.Request
	task.Grant.ID = "new"
	if ApprovedFor(a, task, r) {
		t.Fatal("previous grant accepted")
	}
}
