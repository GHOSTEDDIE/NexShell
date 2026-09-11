package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *App) welcome() fyne.CanvasObject {
	return panel(container.NewCenter(container.NewVBox(headingText("连接服务器，开始工作"), metaText("在连接列表中选择一台服务器"), gap(12), action("查看连接列表", designIcon("server"), func() { u.sideTabs.SelectIndex(1) }))), theme.ColorNameBackground, false, 0)
}
func (u *App) connectionsPane() fyne.CanvasObject {
	u.homeCount = widget.NewLabel("已保存连接")
	u.search = widget.NewEntry()
	u.search.SetPlaceHolder("搜索连接")
	u.search.OnChanged = func(string) { u.filterHosts() }
	u.homeList = widget.NewList(func() int { return len(u.filtered) }, func() fyne.CanvasObject { return newConnectionRow() }, func(i widget.ListItemID, obj fyne.CanvasObject) {
		h := u.filtered[i]
		row := obj.(*connectionRow)
		group := ""
		if i == 0 || u.filtered[i-1].Group != h.Group {
			group = h.Group
			if group == "" {
				group = "未分组"
			}
		}
		live := false
		for _, ws := range u.workspaces {
			if ws.host.ID == h.ID && ws.ctx.Err() == nil {
				live = true
				break
			}
		}
		row.setHost(h, group, live, h.ID == u.selected)
		row.count.SetText(fmt.Sprint(u.groupCounts[h.Group]))
		row.content.Refresh()
		row.selectHost = func() { u.selected = h.ID; u.homeList.Refresh() }
		row.connectHost = func() { u.connect(h) }
		row.onMenu = func(pos fyne.Position) {
			u.selected = h.ID
			widget.NewPopUpMenu(fyne.NewMenu("", fyne.NewMenuItem("连接", func() { u.connect(h) }), fyne.NewMenuItem("编辑", func() { u.editHost(&h) }), fyne.NewMenuItem("删除", u.deleteHost)), u.Window.Canvas()).ShowAtPosition(pos)
		}
	})
	u.homeList.HideSeparators = true
	bottom := container.NewVBox(outlineAction("新建连接", designIcon("plus"), func() { u.editHost(nil) }), action("导入连接", nil, u.importHosts))
	return inset(container.NewBorder(inset(u.search, 0, 0, 14, 0), inset(bottom, 12, 0, 0, 0), nil, nil, u.homeList), 18, 14, 18, 14)
}
func (u *App) desktop(side fyne.CanvasObject) fyne.CanvasObject {
	u.sideTabs = newTabView(false, container.NewTabItem("服务器状态", side), container.NewTabItem("连接列表", u.connectionsPane()))
	u.assistantVisible = true
	u.leftWidth = savedPanelSize(u.UI.Preferences(), "layout.leftWidth", 248)
	u.rightWidth = savedPanelSize(u.UI.Preferences(), "layout.rightWidth", 340)
	saveWidths := func() {
		u.UI.Preferences().SetFloat("layout.leftWidth", float64(u.leftWidth))
		u.UI.Preferences().SetFloat("layout.rightWidth", float64(u.rightWidth))
	}
	leftDivider := newResizeDivider(false, func(delta float32) {
		right := float32(0)
		gutters := dividerSize
		if u.assistantVisible {
			right = u.desktopBody.Objects[2].Size().Width
			u.rightWidth = right
			gutters += dividerSize
		}
		u.leftWidth = boundedPanelSize(u.desktopBody.Objects[0].Size().Width+delta, minimumLeftWidth, u.desktopBody.Size().Width-right-minimumCenterWidth-gutters)
		u.desktopBody.Refresh()
	}, saveWidths)
	rightDivider := newResizeDivider(false, func(delta float32) {
		if !u.assistantVisible {
			return
		}
		u.leftWidth = u.desktopBody.Objects[0].Size().Width
		u.rightWidth = boundedPanelSize(u.desktopBody.Objects[2].Size().Width-delta, minimumRightWidth, u.desktopBody.Size().Width-u.leftWidth-minimumCenterWidth-2*dividerSize)
		u.desktopBody.Refresh()
	}, saveWidths)
	u.desktopBody = container.New(workbenchLayout{u: u}, panel(u.sideTabs, colorPanel, false, 0), u.tabs, panel(u.agentPanel(), colorSoft, false, 0), leftDivider, rightDivider)
	toolbar := container.NewBorder(nil, nil, container.NewHBox(widget.NewIcon(designIcon("terminal")), headingText("NexShell")), container.NewHBox(metaText("服务器工作台"), action("设置", designIcon("settings"), func() { u.settingsDialog("models") }), action("", designIcon("spark"), func() { u.toggleAssistant() })), layout.NewSpacer())
	u.sessionCount = metaText("0 个会话")
	u.status.SizeName = sizeMeta
	u.status.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, u.status, u.sessionCount, layout.NewSpacer())
	quitItem := fyne.NewMenuItem("退出 NexShell", u.quit)
	quitItem.IsQuit = true
	u.Window.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("连接", fyne.NewMenuItem("新建连接", func() { u.editHost(nil) }), fyne.NewMenuItem("导入连接", u.importHosts), fyne.NewMenuItem("导出连接", u.exportHosts), fyne.NewMenuItemSeparator(), quitItem), fyne.NewMenu("工具", fyne.NewMenuItem("批量执行", u.batchDialog), fyne.NewMenuItem("端口转发", u.tunnelDialog), fyne.NewMenuItem("快捷命令", u.snippetsDialog))))
	return edge(sized(panel(inset(toolbar, 6, 16, 6, 16), colorSoft, false, 0), 0, 44), sized(panel(inset(footer, 0, 18, 0, 18), theme.ColorNameBackground, true, 0), 0, 28), nil, nil, u.desktopBody)
}
func (u *App) toggleAssistant() {
	u.assistantVisible = !u.assistantVisible
	obj := u.desktopBody.Objects[2]
	if u.assistantVisible {
		obj.Show()
	} else {
		obj.Hide()
	}
	u.desktopBody.Refresh()
}

type workbenchLayout struct{ u *App }

func (l workbenchLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(1100, 580) }
func (l workbenchLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {

	left, center, right := fitWorkbenchWidths(s.Width, l.u.leftWidth, l.u.rightWidth, l.u.assistantVisible)
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(fyne.NewSize(left, s.Height))
	o[3].Move(fyne.NewPos(left, 0))
	o[3].Resize(fyne.NewSize(dividerSize, s.Height))
	centerX := left + dividerSize
	o[1].Move(fyne.NewPos(centerX, 0))
	o[1].Resize(fyne.NewSize(center, s.Height))
	if l.u.assistantVisible {
		o[4].Show()
		o[4].Move(fyne.NewPos(centerX+center, 0))
		o[4].Resize(fyne.NewSize(dividerSize, s.Height))
		o[2].Move(fyne.NewPos(centerX+center+dividerSize, 0))
		o[2].Resize(fyne.NewSize(right, s.Height))
	} else {
		o[4].Hide()
	}

}
func (u *App) refreshDesktopState() {
	if u.sessionCount != nil {
		u.sessionCount.SetText(fmt.Sprintf("%d 个会话", len(u.workspaces)))
	}
	if u.homeList != nil {
		u.homeList.Refresh()
	}
}

func (u *App) showWorkspace(ws *workspace) {
	u.workspaces = append(u.workspaces, ws)
	u.tabs.Append(ws.tab)
	u.tabs.Select(ws.tab)
	u.tabs.Remove(u.homeTab)
}
