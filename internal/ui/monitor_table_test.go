package ui

import (
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"testing"
)

func TestMonitorTableKeepsStructuredSelectionAndClearsErrors(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	m := newMonitorTable("进程", nil)
	m.update(remote.Metric{State: "ok", Value: "PID USER %CPU %MEM COMMAND\n1 root 150.0 2.0 first worker\n2 app 50.0 1.0 second"})
	if len(m.rows) != 2 || m.rows[0][0] != "first worker" || m.rows[0][3] != "150.0%" || !m.table.ShowHeaderRow {
		t.Fatal("process columns not structured", m.rows)
	}
	m.table.Select(widget.TableCellID{Row: 0, Col: 0})
	m.update(remote.Metric{State: "ok", Value: "PID USER %CPU %MEM COMMAND\n2 app 160.0 1.0 second\n1 root 120.0 2.0 first worker"})
	if m.selected != "1" || !m.detail.Visible() {
		t.Fatal("selection lost when sort changed")
	}
	m.update(remote.Metric{State: "error", Error: "permission denied"})
	if len(m.rows) != 0 || m.table.Visible() || m.selected != "" || m.message.Text != "采集失败" {
		t.Fatal("stale rows not cleared")
	}
	m.update(remote.Metric{State: "ok", Value: "bad data"})
	if m.table.Visible() || m.message.Text == "" {
		t.Fatal("parse failure hidden")
	}
}
func TestPortTableSeparatesAddressesAndPort(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	m := newMonitorTable("端口连接", nil)
	m.update(remote.Metric{State: "ok", Value: "tcp LISTEN 0 4096 [::]:8080 [::]:*"})
	if len(m.rows) != 1 || m.rows[0][1] != "8080" || m.rows[0][2] != "监听" || m.rows[0][3] != "[::]:8080" || m.rows[0][4] != "[::]:*" {
		t.Fatal("port columns shifted", m.rows)
	}
}
