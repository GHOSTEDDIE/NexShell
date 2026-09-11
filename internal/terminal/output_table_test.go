package terminal

import (
	"github.com/charmbracelet/x/ansi"
	"reflect"
	"strings"
	"testing"
)

func TestDockerTableSurvivesWrappingAndResize(t *testing.T) {
	headers := []string{"CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS", "NAMES"}
	rows := [][]string{{"abc123", "registry.example.com/app:v2", `"sh -c 'echo  hello 世界🔧'"`, "2 hours ago", "Up 2 hours", "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp", "backend"}, {"def456", "redis:7", `"redis-server"`, "3 hours ago", "Up 3 hours", "", "cache"}}
	widths := []int{16, 32, 40, 20, 24, 44}
	line := func(row []string) string {
		var b strings.Builder
		for i, c := range row {
			b.WriteString(c)
			if i < len(widths) {
				b.WriteString(strings.Repeat(" ", widths[i]-ansi.StringWidth(c)))
			}
		}
		return b.String()
	}
	core := NewCore(40, 8)
	defer core.CloseInput()
	core.Write([]byte(line(headers) + "\r\n" + line(rows[0]) + "\r\n" + line(rows[1]) + "\r\nroot@host:~# "))
	for _, cols := range []int{40, 120, 24, 240, 80} {
		core.Resize(cols, 8)
		table, ok := core.LatestTable()
		if !ok || !reflect.DeepEqual(table.Headers, headers) || !reflect.DeepEqual(table.Rows, rows) {
			t.Fatalf("table damaged at %d columns: %+v", cols, table)
		}
	}
}
func TestPlainOutputIsNotATable(t *testing.T) {
	if _, ok := ParseOutputTable([]string{"a normal sentence", "some  spaced  text"}); ok {
		t.Fatal("plain output misclassified")
	}
}
