package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestContextMenuInsetsActionsAndEdgePlacement(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	w := a.NewWindow("menu")
	defer w.Close()
	entry := widget.NewEntry()
	button := action("", designIcon("more"), nil)
	w.SetContent(container.NewBorder(entry, container.NewHBox(button), nil, nil, widget.NewLabel("工作台")))
	w.Resize(fyne.NewSize(560, 500))
	w.Show()
	w.Canvas().Focus(entry)
	calls := 0
	for _, dark := range []bool{false, true} {
		for _, scale := range []float64{1, 1.3} {
			a.Preferences().SetFloat("appearance.textScale", scale)
			setAppearance(a, dark)
			check := fyne.NewMenuItem("跟随终端目录", func() { calls++ })
			check.Checked = true
			data := fyne.NewMenu("", fyne.NewMenuItem("打开", func() { calls++ }), fyne.NewMenuItem("下载", func() { calls++ }), fyne.NewMenuItem("重命名", func() {}), fyne.NewMenuItem("权限", func() {}), fyne.NewMenuItem("压缩", func() {}), fyne.NewMenuItem("删除", func() {}), fyne.NewMenuItemSeparator(), check, fyne.NewMenuItem("终端进入目录", func() {}))
			p := showContextMenu(data, w.Canvas(), fyne.NewPos(550, 490))
			origin := a.Driver().AbsolutePositionForObject(p.menu)
			if origin.X-p.Position().X < 11 || origin.Y-p.Position().Y < 6 {
				t.Fatal("menu outer padding missing")
			}
			if p.Position().X < menuEdgeMargin || p.Position().Y < menuEdgeMargin || p.Position().X+p.Size().Width > w.Canvas().Size().Width-menuEdgeMargin+.1 || p.Position().Y+p.Size().Height > w.Canvas().Size().Height-menuEdgeMargin+.1 {
				t.Fatalf("menu touches window edge: %v %v", p.Position(), p.Size())
			}
			last := p.menu.Items[len(p.menu.Items)-1]
			bottom := a.Driver().AbsolutePositionForObject(last).Y + last.Size().Height
			if p.Position().Y+p.Size().Height-bottom < 6 {
				t.Fatal("bottom row touches surface edge")
			}
			p.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
			if dir := os.Getenv("NEXSHELL_MENU_EVIDENCE"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, fmt.Sprintf("menu-dark-%t-scale-%.1f.png", dark, scale)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, w.Canvas().Capture())
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			before := calls
			textX := func(menu *widget.Menu) float32 {
				for _, o := range test.WidgetRenderer(menu.Items[0].(fyne.Widget)).Objects() {
					if text, ok := o.(*canvas.Text); ok {
						return text.Position().X
					}
				}
				t.Fatal("menu text missing")
				return 0
			}
			checkedX := textX(p.menu)
			p.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if calls != before+1 || len(w.Canvas().Overlays().List()) != 0 {
				t.Fatal("menu selection did not execute once and close")
			}
			if w.Canvas().Focused() != entry {
				t.Fatal("menu did not restore the prior focus context")
			}
			check.Checked = false
			p = showContextMenu(data, w.Canvas(), fyne.NewPos(200, 100))
			if textX(p.menu) != checkedX {
				t.Fatal("turning check off shifted menu text")
			}
			if data.Items[0].Icon != nil {
				t.Fatal("menu styling mutated caller data")
			}
			p.Hide()
			p = showActionMenu(data, w.Canvas(), button)
			buttonY := a.Driver().AbsolutePositionForObject(button).Y
			if p.Position().Y+p.Size().Height > buttonY {
				t.Fatal("bottom toolbar menu did not open upwards")
			}
			p.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			if len(w.Canvas().Overlays().List()) != 0 {
				t.Fatal("Escape did not dismiss menu")
			}
			p = showContextMenu(data, w.Canvas(), fyne.NewPos(550, 490))
			w.Resize(fyne.NewSize(520, 460))
			if p.Position().X+p.Size().Width > w.Canvas().Size().Width-menuEdgeMargin+.1 || p.Position().Y+p.Size().Height > w.Canvas().Size().Height-menuEdgeMargin+.1 {
				t.Fatal("window resize lost menu edge margin")
			}
			test.TapCanvas(w.Canvas(), fyne.NewPos(12, 12))
			if p.Visible() || len(w.Canvas().Overlays().List()) != 0 {
				t.Fatal("outside click did not dismiss menu")
			}
			w.Resize(fyne.NewSize(560, 500))
		}
	}
}
