package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandTableFullValueAndRendering(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	w := a.NewWindow("table")
	w.Resize(fyne.NewSize(1200, 800))
	w.Show()
	defer w.Close()
	data := terminal.OutputTable{Headers: []string{"CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS", "NAMES"}, Rows: [][]string{{"abc123", "registry.example.com/app:v2", `"sh -c 'echo  hello 世界'"`, "2 hours ago", "Up 2 hours", "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp", "backend"}, {"def456", "redis:7", `"redis-server"`, "3 hours ago", "Up 3 hours", "", "cache"}}}
	d := showCommandTable(data, w)
	var table *widget.Table
	var detail *widget.RichText
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Table:
			table = v
		case *widget.RichText:
			detail = v
		case *fyne.Container:
			for _, child := range v.Objects {
				walk(child)
			}
		case *surface:
			walk(v.Content)
		case *container.Scroll:
			walk(v.Content)
		}
	}
	walk(d.popup.Content)

	if table == nil {
		t.Fatal("table view missing")
	}
	table.Select(widget.TableCellID{Row: 0, Col: 2})
	if detail == nil || len(detail.Segments) != 1 {
		t.Fatal("full cell detail missing")
	}
	if value, ok := detail.Segments[0].(*widget.TextSegment); !ok || !strings.Contains(value.Text, data.Rows[0][2]) {
		t.Fatal("full command text lost")
	}
	if rows, cols := table.Length(); rows != 2 || cols != 7 {
		t.Fatal("table dimensions changed")
	}
	if dir := os.Getenv("NEXSHELL_TABLE_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "docker-table.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}
