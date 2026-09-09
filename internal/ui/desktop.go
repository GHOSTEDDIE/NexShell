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
	u.desktopBody = container.New(workbenchLayout{u: u}, panel(u.sideTabs, colorPanel, false, 0), u.tabs, panel(u.agentPanel(), colorSoft, false, 0))
	toolbar := container.NewBorder(nil, nil, container.NewHBox(widget.NewIcon(designIcon("terminal")), headingText("NexShell")), container.NewHBox(metaText("服务器工作台"), action("设置", designIcon("settings"), func() { u.settingsDialog("models") }), action("", designIcon("spark"), func() { u.toggleAssistant() })), layout.NewSpacer())
	u.sessionCount = metaText("0 个会话")
	u.status.SizeName = sizeMeta
	u.status.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, u.status, u.sessionCount, layout.NewSpacer())
	u.Window.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("连接", fyne.NewMenuItem("新建连接", func() { u.editHost(nil) }), fyne.NewMenuItem("导入连接", u.importHosts), fyne.NewMenuItem("导出连接", u.exportHosts)), fyne.NewMenu("工具", fyne.NewMenuItem("批量执行", u.batchDialog), fyne.NewMenuItem("端口转发", u.tunnelDialog), fyne.NewMenuItem("快捷命令", u.snippetsDialog))))
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
	left, right := float32(248), float32(340)
	if s.Width >= 1600 {
		left = 264
		right = 372
	}
	if !l.u.assistantVisible {
		right = 0
	}
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(fyne.NewSize(left, s.Height))
	o[1].Move(fyne.NewPos(left, 0))
	o[1].Resize(fyne.NewSize(max(240, s.Width-left-right), s.Height))
	if right > 0 {
		o[2].Move(fyne.NewPos(s.Width-right, 0))
		o[2].Resize(fyne.NewSize(right, s.Height))
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
