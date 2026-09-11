package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Keep Fyne's menu items, checks, submenus and keyboard handling. Only the
// surrounding surface, row spacing and window-edge positioning are customized.
type contextMenu struct {
	*widget.PopUp
	menu *widget.Menu
}

type menuTheme struct{ componentTheme }

func (t menuTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameMenuBackground || n == theme.ColorNameShadow {
		return color.Transparent
	}
	if n == theme.ColorNameFocus {
		return t.componentTheme.Color(theme.ColorNameSelection, v)
	}
	return t.componentTheme.Color(n, v)
}
func (t menuTheme) Size(n fyne.ThemeSizeName) float32 {
	scale := t.componentTheme.Size(sizeControl) / 13
	switch n {
	case theme.SizeNamePadding:
		return 2 * scale
	case theme.SizeNameInnerPadding:
		return 4 * scale
	case theme.SizeNameInlineIcon:
		return 16 * scale
	case theme.SizeNameMenuRadius:
		return 5 * scale
	}
	return t.componentTheme.Size(n)
}

func newContextMenu(data *fyne.Menu, c fyne.Canvas) *contextMenu {
	menu := widget.NewMenu(menuWithCheckGutter(data))
	scale := theme.Size(sizeControl) / 13
	content := container.NewThemeOverride(menu, menuTheme{})
	body := inset(content, 7*scale, 12*scale, 7*scale, 12*scale)
	// Extend before the base widget is initialized, so overlay resize and
	// keyboard focus are dispatched to this menu rather than the embedded popup.
	p := &contextMenu{PopUp: &widget.PopUp{Content: body, Canvas: c}, menu: menu}
	p.ExtendBaseWidget(p)
	menu.OnDismiss = p.Hide
	p.Resize(p.MinSize())
	return p
}

var menuGutterIcon = fyne.NewStaticResource("menu-gutter.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path d="M0 0h16v16H0z" fill="none"/></svg>`))

// Fyne reserves a check column only when a row is checked. Keep the same
// gutter when all checks are off, without mutating the caller's menu items.
func menuWithCheckGutter(data *fyne.Menu) *fyne.Menu {
	for _, item := range data.Items {
		if item.Checked {
			return data
		}
	}
	copyMenu := *data
	copyMenu.Items = make([]*fyne.MenuItem, len(data.Items))
	for i, item := range data.Items {
		copyItem := *item
		if !item.IsSeparator && item.Icon == nil {
			copyItem.Icon = menuGutterIcon
		}
		copyMenu.Items[i] = &copyItem
	}
	return &copyMenu
}

const menuEdgeMargin float32 = 10

func menuPosition(c fyne.Canvas, pos fyne.Position, size fyne.Size) fyne.Position {
	origin, area := c.InteractiveArea()
	return fyne.NewPos(
		max(origin.X+menuEdgeMargin, min(pos.X, origin.X+area.Width-size.Width-menuEdgeMargin)),
		max(origin.Y+menuEdgeMargin, min(pos.Y, origin.Y+area.Height-size.Height-menuEdgeMargin)),
	)
}
func (p *contextMenu) ShowAtPosition(pos fyne.Position) {
	p.PopUp.ShowAtPosition(menuPosition(p.Canvas, pos, p.MinSize()))
	p.Canvas.Focus(p)
}
func (p *contextMenu) Resize(size fyne.Size) {
	p.PopUp.Resize(size)
	p.PopUp.Move(menuPosition(p.Canvas, p.Position(), p.Size()))
}
func (p *contextMenu) FocusGained()   {}
func (p *contextMenu) FocusLost()     {}
func (p *contextMenu) TypedRune(rune) {}
func (p *contextMenu) TypedKey(e *fyne.KeyEvent) {
	// PopUpMenu's keyboard handler delegates to the public Menu navigation API.
	(&widget.PopUpMenu{Menu: p.menu}).TypedKey(e)
}

func showContextMenu(menu *fyne.Menu, c fyne.Canvas, pos fyne.Position) *contextMenu {
	p := newContextMenu(menu, c)
	p.ShowAtPosition(pos)
	return p
}

func showActionMenu(menu *fyne.Menu, c fyne.Canvas, button fyne.CanvasObject) *contextMenu {
	p := newContextMenu(menu, c)
	origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(button)
	pos := origin.Add(fyne.NewPos(0, button.Size().Height+6))
	areaOrigin, area := c.InteractiveArea()
	if pos.Y+p.MinSize().Height > areaOrigin.Y+area.Height-menuEdgeMargin {
		pos.Y = origin.Y - p.MinSize().Height - 6
	}
	p.ShowAtPosition(pos)
	return p
}
