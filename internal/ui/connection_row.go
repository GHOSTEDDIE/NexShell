package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
)

type connectionRow struct {
	widget.BaseWidget
	name, detail, group, live *textView
	count                     *textView
	groupHeader               *fyne.Container
	content                   *fyne.Container
	card                      *surface
	selectHost, connectHost   func()
	onMenu                    func(fyne.Position)
}

func newConnectionRow() *connectionRow {
	r := &connectionRow{name: headingText("名称"), detail: metaText("地址"), group: metaText(""), live: textUI("●", sizeMeta, theme.ColorNameSuccess, false)}
	row := container.NewBorder(nil, nil, inset(widget.NewIcon(designIcon("server")), 0, 9, 0, 0), r.live, container.NewVBox(r.name, r.detail))
	r.card = panel(inset(row, 10, 10, 10, 10), colorPanel, false, 6)
	r.count = metaText("")
	r.groupHeader = container.NewBorder(nil, nil, sized(widget.NewIcon(designIcon("chevron")), 14, 18), r.count, r.group)
	r.groupHeader.Hide()
	r.content = container.NewVBox(r.groupHeader, r.card)
	r.ExtendBaseWidget(r)
	return r
}
func (r *connectionRow) setHost(h domain.Host, group string, live, selected bool) {
	r.name.SetText(h.Name)
	r.detail.SetText(h.Address)
	r.group.SetText(group)
	if group == "" {
		r.groupHeader.Hide()
	} else {
		r.groupHeader.Show()
	}
	if live {
		r.live.Show()
	} else {
		r.live.Hide()
	}
	r.card.tone = colorPanel
	r.name.tone = theme.ColorNameForeground
	if selected {
		r.card.tone = theme.ColorNameSelection
		r.name.tone = theme.ColorNamePrimary
	}
	r.card.Refresh()
}
func (r *connectionRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}
func (r *connectionRow) Tapped(*fyne.PointEvent) {
	if r.selectHost != nil {
		r.selectHost()
	}
}
func (r *connectionRow) DoubleTapped(*fyne.PointEvent) {
	if r.selectHost != nil {
		r.selectHost()
	}
	if r.connectHost != nil {
		r.connectHost()
	}
}
func (r *connectionRow) TappedSecondary(e *fyne.PointEvent) {
	if r.onMenu != nil {
		r.onMenu(e.AbsolutePosition)
	}
}
