package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/charmbracelet/x/ansi"
)

func (w *workspace) showOutputTable() {
	view := w.terminal
	for i, session := range w.sessions {
		if session == w.activeSession.Load() && i < len(w.terminals) {
			view = w.terminals[i]
			break
		}
	}
	data, ok := view.Core.LatestTable()
	if !ok {
		showMotionInformation("表格查看", "当前输出中没有可识别的表格。请先运行 docker ps、kubectl get 等表格命令。", w.u.Window)
		return
	}
	showCommandTable(data, w.u.Window)
}
func showCommandTable(data terminal.OutputTable, parent fyne.Window) *motionDialog {
	cell := func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.SizeName = sizeControl
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	table := widget.NewTable(func() (int, int) { return len(data.Rows), len(data.Headers) }, cell, func(id widget.TableCellID, o fyne.CanvasObject) { o.(*widget.Label).SetText(data.Rows[id.Row][id.Col]) })
	table.ShowHeaderRow = true
	table.CreateHeader = cell
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col < 0 {
			return
		}
		l := o.(*widget.Label)
		l.TextStyle.Bold = true
		l.SetText(data.Headers[id.Col])
	}
	for col, header := range data.Headers {
		width := ansi.StringWidth(header)
		for _, row := range data.Rows {
			width = max(width, ansi.StringWidth(row[col]))
		}
		table.SetColumnWidth(col, max(100, min(420, float32(width)*8+24)))
	}
	detail := widget.NewRichTextWithText("点击单元格查看完整内容")
	detail.Wrapping = fyne.TextWrapWord
	selected := ""
	copyCell := outlineAction("复制单元格", nil, func() { parent.Clipboard().SetContent(selected) })
	copyCell.Disable()
	table.OnSelected = func(id widget.TableCellID) {
		selected = data.Rows[id.Row][id.Col]
		detail.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: data.Headers[id.Col] + "：" + selected, Style: widget.RichTextStyle{Inline: true}}}
		detail.Refresh()
		copyCell.Enable()
	}
	tools := container.NewBorder(nil, nil, metaText(fmt.Sprintf("%d 行 · %d 列", len(data.Rows), len(data.Headers))), container.NewHBox(copyCell, outlineAction("复制表格", nil, func() { parent.Clipboard().SetContent(data.TSV()) })))
	body := edge(tools, sized(container.NewVScroll(detail), 0, 80), nil, nil, table)
	d := newMotionDialog("终端表格", "关闭", body, parent)
	d.Resize(fyne.NewSize(min(1100, parent.Canvas().Size().Width-60), min(650, parent.Canvas().Size().Height-60)))
	d.Show()
	return d
}
