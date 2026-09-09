package remote

import (
	"reflect"
	"testing"
)

func TestProcessColumnsKeepMultiCoreUsageAndNames(t *testing.T) {
	got, e := ParseProcesses("  PID USER %CPU %MEM COMMAND\n 1916 9987 250.5 0.8 ts3server\n 620 root 0.2 1.0 service  worker\n")
	want := []ProcessInfo{{PID: 1916, User: "9987", CPU: 250.5, Memory: .8, Name: "ts3server"}, {PID: 620, User: "root", CPU: .2, Memory: 1, Name: "service  worker"}}
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v %v", got, e)
	}
	for _, s := range []string{"permission denied", "1 root NaN 0 test", "1 root 2 inf test", "0 root 2 0 test"} {
		if _, e := ParseProcesses(s); e == nil {
			t.Fatalf("invalid process data accepted: %q", s)
		}
	}
}
func TestPortColumnsPreserveIPv6AndOwner(t *testing.T) {
	text := "Netid State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\nudp UNCONN 0 0 127.0.0.53%lo:53 0.0.0.0:*\ntcp LISTEN 0 4096 [::]:22 [::]:* users:((\"sshd\",pid=834,fd=3))\ntcp ESTAB 12 20 [::ffff:127.0.0.1]:443 [::ffff:127.0.0.2]:52000 users:((\"app worker\",pid=45,fd=8))\n"
	rows, e := ParsePorts(text)
	if e != nil || len(rows) != 3 {
		t.Fatal(rows, e)
	}
	if len(rows[1].Owners) != 1 || rows[1].Owners[0].Name != "sshd" || rows[1].Owners[0].PID != 834 || rows[2].Owners[0].Name != "app worker" {
		t.Fatalf("owner fields missing: %+v", rows)
	}
	if rows[0].Process != "" || rows[1].Local != "[::]:22" || rows[2].ReceiveQueue != 12 || rows[2].Process != `users:(("app worker",pid=45,fd=8))` {
		t.Fatalf("columns shifted: %+v", rows)
	}
	for _, s := range []string{"permission denied", "tcp LISTEN no 2 *:22 *:*", "tcp LISTEN 0 2 *:22"} {
		if _, e := ParsePorts(s); e == nil {
			t.Fatalf("invalid port data accepted %q", s)
		}
	}
}
func TestEmptyMonitorTablesAreValid(t *testing.T) {
	p, e := ParseProcesses("PID USER %CPU %MEM COMMAND\n")
	if e != nil || len(p) != 0 {
		t.Fatal(p, e)
	}
	ports, e := ParsePorts("Netid State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\n")
	if e != nil || len(ports) != 0 {
		t.Fatal(ports, e)
	}
}
