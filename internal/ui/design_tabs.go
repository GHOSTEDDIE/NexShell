package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// tabView shares the same owner callbacks as DocTabs but keeps the prototype's
// 38px tab strip and preserves the selected content when another pane changes.
type tabView struct {
	Pinned *container.TabItem
	widget.BaseWidget
	Items                                []*container.TabItem
	OnSelected, OnClosed, CloseIntercept func(*container.TabItem)
	CreateTab                            func() *container.TabItem
	current                              *container.TabItem
	document                             bool
	strip, body                          *fyne.Container
	content                              fyne.CanvasObject
}

func newTabView(document bool, items ...*container.TabItem) *tabView {
	t := &tabView{document: document, strip: container.NewHBox(), body: container.NewStack()}
	t.ExtendBaseWidget(t)
	bar := sized(panel(container.NewHScroll(t.strip), colorPanel, false, 0), 0, 38)
	t.content = edge(bar, nil, nil, nil, t.body)
	for _, item := range items {
		t.Append(item)
	}
	return t
}
func (t *tabView) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
func (t *tabView) Selected() *container.TabItem        { return t.current }
func (t *tabView) Select(item *container.TabItem) {
	found := false
	for _, v := range t.Items {
		if v == item {
			found = true
			break
		}
	}
	if !found || item == t.current {
		return
	}
	t.current = item
	t.rebuild()
	if t.OnSelected != nil {
		t.OnSelected(item)
	}
}
func (t *tabView) SelectIndex(i int) {
	if i >= 0 && i < len(t.Items) {
		t.Select(t.Items[i])
	}
}
func (t *tabView) Append(item *container.TabItem) {
	t.Items = append(t.Items, item)
	if t.current == nil {
		t.current = item
	}
	t.rebuild()
}
func (t *tabView) Remove(item *container.TabItem) {
	for i, v := range t.Items {
		if v != item {
			continue
		}
		copy(t.Items[i:], t.Items[i+1:])
		t.Items[len(t.Items)-1] = nil
		t.Items = t.Items[:len(t.Items)-1]
		if t.current == item {
			t.current = nil
			if len(t.Items) > 0 {
				t.current = t.Items[max(0, i-1)]
			}
		}
		t.rebuild()
		if t.OnSelected != nil {
			t.OnSelected(t.current)
		}
		break
	}
}
func (t *tabView) Refresh() { t.rebuild(); t.BaseWidget.Refresh() }
func (t *tabView) rebuild() {
	t.strip.Objects = nil
	for _, item := range t.Items {
		item := item
		b := action(item.Text, item.Icon, func() { t.Select(item) })
		b.tab = true
		b.document = t.document
		b.selected = item == t.current
		var obj fyne.CanvasObject = b
		if t.document && item != t.Pinned {
			close := action("", designIcon("close"), func() {
				if t.CloseIntercept != nil {
					t.CloseIntercept(item)
				} else {
					t.Remove(item)
					if t.OnClosed != nil {
						t.OnClosed(item)
					}
				}
			})
			obj = container.NewHBox(b, close)
		}
		t.strip.Add(obj)
	}
	if t.CreateTab != nil {
		t.strip.Add(action("", designIcon("plus"), func() {
			if item := t.CreateTab(); item != nil {
				t.Append(item)
				t.Select(item)
			}
		}))
	}
	t.body.Objects = nil
	if t.current != nil && t.current.Content != nil {
		t.body.Objects = []fyne.CanvasObject{t.current.Content}
	}
	t.body.Refresh()
	t.strip.Refresh()
}
