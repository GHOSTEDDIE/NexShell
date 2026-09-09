package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestDiskTableReadableAndRefreshesSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setAppearance(app, false)
	d := newDiskTable()
	win := app.NewWindow("磁盘占用")
	defer win.Close()
	win.SetContent(d.content)
	win.Resize(fyne.NewSize(260, 380))
	win.Show()

	sample := "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/root 10485760 4194304 5767168 43% /\n/dev/mapper/data 209715200 104857600 104857600 50% /srv/shared files\ntmpfs 1024 0 1024 0% /run\n"
	d.update(remote.Metric{State: "ok", Value: sample})
	rows, cols := d.table.Length()
	if rows != 3 || cols != 5 {
		t.Fatal(rows, cols)
	}
	cells := diskCells(d.rows[0])
	if cells[1] != "43%" || cells[2] != "5.5GiB" || cells[3] != "4.0GiB" || cells[4] != "10.0GiB" {
		t.Fatal(cells)
	}
	d.table.Select(widget.TableCellID{Row: 1, Col: 0})
	d.update(remote.Metric{State: "ok", Value: strings.Replace(sample, "50%", "51%", 1)})
	if !d.detail.Visible() || !strings.Contains(d.detail.Text, "/srv/shared files") || !strings.Contains(d.detail.Text, "51%") {
		t.Fatal("selection detail lost on refresh", d.detail.Text)
	}
	if p := os.Getenv("NEXSHELL_DISK_SCREENSHOT"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	d.update(remote.Metric{State: "error", Error: "permission denied"})
	if len(d.rows) != 0 || d.detail.Visible() || !d.message.Visible() {
		t.Fatal("failed sample left valid-looking old disks")
	}
	if metricState(remote.Metric{State: "warming_up"}) != "—" {
		t.Fatal("sampling explanation is still visible")
	}
}
