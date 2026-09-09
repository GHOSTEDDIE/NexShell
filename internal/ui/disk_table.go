package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
)

type diskTable struct {
	detailScroll    *container.Scroll
	selectedMount   string
	rows            []remote.DiskUsage
	table           *widget.Table
	message, detail *widget.Label
	content         fyne.CanvasObject
}

func newDiskTable() *diskTable {
	d := &diskTable{message: widget.NewLabel(""), detail: widget.NewLabel("")}
	d.message.Wrapping = fyne.TextWrapWord
	d.detail.Wrapping = fyne.TextWrapWord
	d.detailScroll = container.NewVScroll(d.detail)
	d.detailScroll.SetMinSize(fyne.NewSize(0, 84))
	d.detailScroll.Hide()
	d.message.Hide()
	d.detail.Hide()
	d.detailScroll.Hide()
	d.table = widget.NewTable(func() (int, int) { return len(d.rows), 5 }, func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Truncation = fyne.TextTruncateEllipsis
		return label
	}, func(id widget.TableCellID, obj fyne.CanvasObject) {
		label := obj.(*widget.Label)
		label.Alignment = fyne.TextAlignTrailing
		if id.Col == 0 {
			label.Alignment = fyne.TextAlignLeading
		}
		label.SetText(diskCells(d.rows[id.Row])[id.Col])
	})
	headers := []string{"挂载点", "占用", "可用", "已用", "总量"}
	d.table.ShowHeaderRow = true
	d.table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	d.table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) { obj.(*widget.Label).SetText(headers[id.Col]) }
	for i, width := range []float32{86, 52, 66, 66, 66} {
		d.table.SetColumnWidth(i, width)
	}
	d.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Row >= len(d.rows) {
			return
		}
		r := d.rows[id.Row]
		d.selectedMount = r.Mount
		d.detail.SetText(fmt.Sprintf("%s\n%s\n已用 %s / 总量 %s · 可用 %s · %d%%", r.Mount, r.Filesystem, diskSize(float64(r.UsedKiB)), diskSize(float64(r.TotalKiB)), diskSize(float64(r.AvailableKiB)), r.Percent))
		d.detail.Show()
		d.detailScroll.Show()
		d.content.Refresh()
	}
	d.content = container.NewBorder(container.NewVBox(widget.NewLabelWithStyle("磁盘占用", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), d.message), d.detailScroll, nil, nil, d.table)
	return d
}
func diskSize(kib float64) string {
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	i := 0
	for (kib >= 1024 || kib <= -1024) && i < len(units)-1 {
		kib /= 1024
		i++
	}
	if i == 0 || kib >= 100 || kib <= -100 {
		return fmt.Sprintf("%.0f%s", kib, units[i])
	}
	return fmt.Sprintf("%.1f%s", kib, units[i])
}
func diskCells(r remote.DiskUsage) [5]string {
	return [5]string{r.Mount, fmt.Sprintf("%d%%", r.Percent), diskSize(float64(r.AvailableKiB)), diskSize(float64(r.UsedKiB)), diskSize(float64(r.TotalKiB))}
}
func (d *diskTable) unavailable(message string) {
	d.selectedMount = ""
	d.rows = nil
	d.table.UnselectAll()
	d.table.Refresh()
	d.detail.Hide()
	d.detailScroll.Hide()
	d.message.SetText(message)
	d.message.Show()
	d.content.Refresh()
}
func (d *diskTable) update(metric remote.Metric) {
	if metric.State != "ok" {
		d.unavailable(metricState(metric))
		return
	}
	output, ok := metric.Value.(string)
	if !ok {
		d.unavailable("磁盘数据格式错误")
		return
	}
	rows, err := remote.ParseDiskUsage(output)
	if err != nil {
		d.unavailable(err.Error())
		return
	}
	mount := d.selectedMount
	d.selectedMount = ""
	d.rows = rows
	d.table.UnselectAll()
	d.table.Refresh()
	d.detail.Hide()
	d.detailScroll.Hide()
	for i, row := range rows {
		if row.Mount == mount {
			d.table.Select(widget.TableCellID{Row: i, Col: 0})
			break
		}
	}
	if len(rows) == 0 {
		d.message.SetText("暂无磁盘")
		d.message.Show()
	} else {
		d.message.Hide()
	}
	d.content.Refresh()
}
