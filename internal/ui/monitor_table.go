package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"strconv"
	"strings"
)

// monitorTable keeps one set of full rows for the compact sidebar, selection
// details and expanded table. Both tables virtualize the full result set.
type monitorTable struct {
	viewport          *fyne.Container
	expandedDialog    dialog.Dialog
	expandedMessage   *textView
	expandedDetail    *widget.RichText
	expandedSelection *container.Scroll
	kind              string
	headers           []string
	compact           []int
	rows              [][]string
	keys              []string
	selected          string
	last              string
	table, expanded   *widget.Table
	message, count    *textView
	detail            *widget.RichText
	content           fyne.CanvasObject
	open              *actionButton
	parent            fyne.Window
}

func newMonitorTable(kind string, parent fyne.Window) *monitorTable {
	m := &monitorTable{kind: kind, parent: parent, message: metaText("等待采集"), count: metaText(""), detail: widget.NewRichText()}
	m.detail.Wrapping = fyne.TextWrapWord
	m.detail.Hide()
	if kind == "进程" {
		m.headers = []string{"进程", "进程 ID", "用户", "CPU", "内存"}
		m.compact = []int{0, 3, 4}
	} else {
		m.headers = []string{"协议", "本地端口", "状态", "本地地址", "远端地址", "接收队列", "发送队列", "进程"}
		m.compact = []int{0, 1, 2}
	}
	m.table = m.makeTable(false)
	m.table.Hide()
	m.open = action("查看全部列", designIcon("expand"), m.showExpanded)
	if parent == nil {
		m.open.Disable()
	}
	m.viewport = sized(container.New(&monitorTableLayout{model: m}, m.table), 0, 210)
	m.viewport.Hide()
	m.open.Disable()
	m.content = container.NewVBox(m.message, m.viewport, m.detail, container.NewBorder(nil, nil, m.count, m.open))
	return m
}
func (m *monitorTable) makeTable(full bool) *widget.Table {
	columns := m.compact
	if full {
		columns = make([]int, len(m.headers))
		for i := range columns {
			columns[i] = i
		}
	}
	cell := func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.SizeName = sizeMeta
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	table := widget.NewTable(func() (int, int) { return len(m.rows), len(columns) }, cell, func(id widget.TableCellID, o fyne.CanvasObject) {
		l := o.(*widget.Label)
		l.Alignment = fyne.TextAlignLeading
		if m.numeric(columns[id.Col]) {
			l.Alignment = fyne.TextAlignTrailing
		}
		l.SetText(m.rows[id.Row][columns[id.Col]])
	})
	table.ShowHeaderRow = true
	table.CreateHeader = cell
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		l := o.(*widget.Label)
		l.TextStyle.Bold = true
		l.Alignment = fyne.TextAlignLeading
		if m.numeric(columns[id.Col]) {
			l.Alignment = fyne.TextAlignTrailing
		}
		l.SetText(m.headers[columns[id.Col]])
	}
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(m.rows) {
			m.selected = m.keys[id.Row]
			m.showDetail(id.Row)
		}
	}
	if full {
		widths := []float32{190, 80, 140, 90, 90}
		if m.kind == "端口连接" {
			widths = []float32{58, 70, 80, 180, 180, 76, 76, 180}
		}
		for i, w := range widths {
			table.SetColumnWidth(i, w)
		}
	}
	return table
}
func (m *monitorTable) numeric(col int) bool {
	if m.kind == "进程" {
		return col == 1 || col == 3 || col == 4
	}
	return col == 1 || col == 5 || col == 6
}
func (m *monitorTable) showDetail(row int) {
	segments := []widget.RichTextSegment{}
	for i, label := range m.headers {
		segments = append(segments, &widget.TextSegment{Text: label + "：", Style: widget.RichTextStyle{Inline: true, SizeName: sizeMeta, ColorName: colorMuted}}, &widget.TextSegment{Text: m.rows[row][i] + "\n", Style: widget.RichTextStyle{Inline: true, SizeName: sizeMeta, ColorName: theme.ColorNameForeground}})
	}
	if m.expandedDetail != nil {
		fresh := make([]widget.RichTextSegment, len(segments))
		for i, segment := range segments {
			v := *segment.(*widget.TextSegment)
			fresh[i] = &v
		}
		m.expandedDetail.Segments = fresh
		m.expandedDetail.Refresh()
		m.expandedSelection.Show()
	}
	m.detail.Segments = segments
	m.detail.Show()
	m.detail.Refresh()
	m.content.Refresh()
}
func (m *monitorTable) unavailable(message string) {
	m.last = ""
	m.rows = nil
	m.keys = nil
	m.selected = ""
	m.table.UnselectAll()
	m.table.Refresh()
	m.table.Hide()
	m.viewport.Hide()
	m.open.Disable()
	if m.expanded != nil {
		m.expanded.UnselectAll()
		m.expanded.Refresh()
		if m.expandedMessage != nil {
			m.expandedMessage.SetText(message)
		}
	}
	m.count.SetText("")
	m.detail.Hide()
	if m.expandedSelection != nil {
		m.expandedSelection.Hide()
	}
	m.message.SetText(message)
	m.message.Show()
	m.content.Refresh()
}
func (m *monitorTable) update(metric remote.Metric) {
	if metric.State != "ok" {
		m.unavailable(metricState(metric))
		return
	}
	raw, ok := metric.Value.(string)
	if !ok {
		m.unavailable("采集结果格式错误")
		return
	}
	if raw == m.last && m.table.Visible() {
		return
	}
	rows := [][]string{}
	keys := []string{}
	if m.kind == "进程" {
		values, err := remote.ParseProcesses(raw)
		if err != nil {
			m.unavailable(err.Error())
			return
		}
		for _, v := range values {
			rows = append(rows, []string{v.Name, strconv.Itoa(v.PID), v.User, fmt.Sprintf("%.1f%%", v.CPU), fmt.Sprintf("%.1f%%", v.Memory)})
			keys = append(keys, strconv.Itoa(v.PID))
		}
	} else {
		values, err := remote.ParsePorts(raw)
		if err != nil {
			m.unavailable(err.Error())
			return
		}
		for _, v := range values {
			port := v.Local[strings.LastIndex(v.Local, ":")+1:]
			owner := v.Process
			if len(v.Owners) > 0 {
				names := make([]string, len(v.Owners))
				for i, p := range v.Owners {
					names[i] = fmt.Sprintf("%s (PID %d)", p.Name, p.PID)
				}
				owner = strings.Join(names, ", ")
			}
			if owner == "" {
				owner = "—"
			}
			rows = append(rows, []string{strings.ToUpper(v.Protocol), port, portState(v.State), v.Local, v.Peer, strconv.FormatUint(v.ReceiveQueue, 10), strconv.FormatUint(v.SendQueue, 10), owner})
			keys = append(keys, v.Protocol+"\x00"+v.Local+"\x00"+v.Peer+"\x00"+v.Process)
		}
	}
	selected := m.selected
	m.rows = rows
	m.keys = keys
	m.last = raw
	m.selected = ""
	m.detail.Hide()
	if m.expandedSelection != nil {
		m.expandedSelection.Hide()
	}
	m.table.UnselectAll()
	m.table.Show()
	m.viewport.Show()
	if m.parent != nil && len(rows) > 0 {
		m.open.Enable()
	} else {
		m.open.Disable()
	}
	m.table.Refresh()
	if m.expanded != nil {
		m.expanded.UnselectAll()
		m.expanded.Refresh()
	}
	m.count.SetText(fmt.Sprintf("%d 条", len(rows)))
	if m.expandedMessage != nil {
		m.expandedMessage.SetText(fmt.Sprintf("共 %d 条 · 选择一行可查看完整字段", len(rows)))
	}
	m.message.Hide()
	if len(rows) == 0 {
		m.message.SetText("暂无" + m.kind)
		m.message.Show()
	}
	for i, key := range keys {
		if key == selected {
			m.table.Select(widget.TableCellID{Row: i, Col: 0})
			if m.expanded != nil {
				m.expanded.Select(widget.TableCellID{Row: i, Col: 0})
			}
			break
		}
	}
	m.content.Refresh()
}
func portState(s string) string {
	states := map[string]string{"LISTEN": "监听", "ESTAB": "已连接", "UNCONN": "未连接", "TIME-WAIT": "等待关闭", "CLOSE-WAIT": "等待关闭", "SYN-SENT": "连接中", "SYN-RECV": "握手中"}
	if v, ok := states[s]; ok {
		return v
	}
	return s
}
func (m *monitorTable) showExpanded() {
	if m.parent == nil {
		return
	}
	m.expanded = m.makeTable(true)
	m.expandedMessage = metaText(fmt.Sprintf("共 %d 条 · 选择一行可查看完整字段", len(m.rows)))
	m.expandedDetail = widget.NewRichText()
	m.expandedDetail.Wrapping = fyne.TextWrapWord
	m.expandedSelection = container.NewVScroll(m.expandedDetail)
	m.expandedSelection.SetMinSize(fyne.NewSize(0, 120))
	m.expandedSelection.Hide()
	content := container.NewBorder(m.expandedMessage, m.expandedSelection, nil, nil, m.expanded)
	d := newMotionDialog(m.kind, "关闭", content, m.parent)
	m.expandedDialog = d
	d.SetOnClosed(func() {
		m.expanded = nil
		m.expandedMessage = nil
		m.expandedDialog = nil
		m.expandedDetail = nil
		m.expandedSelection = nil
	})
	d.Resize(fyne.NewSize(min(1080, m.parent.Canvas().Size().Width-60), min(580, m.parent.Canvas().Size().Height-60)))
	d.Show()
}

type monitorTableLayout struct {
	model     *monitorTable
	lastWidth float32
}

func (l *monitorTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(150, 210) }
func (l *monitorTableLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	if l.lastWidth != s.Width {
		l.lastWidth = s.Width
		usable := max(140, s.Width-8)
		widths := []float32{usable - 112, 56, 56}
		if l.model.kind == "端口连接" {
			widths = []float32{48, usable - 114, 66}
		}
		for i, w := range widths {
			l.model.table.SetColumnWidth(i, w)
		}
	}
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(s)
}
