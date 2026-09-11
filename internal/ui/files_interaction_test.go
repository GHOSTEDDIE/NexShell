package ui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"io"
	"strings"
	"testing"
)

func TestTerminalDropReachesUploadValidation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	win := a.NewWindow("drop test")
	defer win.Close()
	v := terminal.NewView(io.Discard, strings.NewReader(""))
	v.Close()
	files := widget.NewLabel("files")
	fileTab := container.NewTabItem("文件", files)
	bottom := newTabView(false, fileTab, container.NewTabItem("传输", widget.NewLabel("transfers")))
	wsTab := container.NewTabItem("server", container.NewBorder(nil, bottom, nil, nil, v))
	u := &App{UI: a, Window: win, tabs: newTabView(false, wsTab)}
	w := &workspace{u: u, ctx: context.Background(), tab: wsTab, terminal: v, terminals: []*terminal.View{v}, fileArea: files, fileTab: fileTab, bottomTabs: bottom, dir: widget.NewEntry()}
	u.workspaces = []*workspace{w}
	win.SetContent(u.tabs)
	win.Resize(fyne.NewSize(900, 600))
	win.Show()
	bottom.Select(bottom.Items[1])
	pos := a.Driver().AbsolutePositionForObject(v).Add(fyne.NewPos(40, 40))
	// A dropped file must reach upload validation even when the bottom pane
	// shows transfers. No directory is ready, so validation must show an error.
	u.filesDropped(pos, []fyne.URI{storage.NewFileURI("/tmp/drop-test.txt")})
	if len(win.Canvas().Overlays().List()) == 0 {
		t.Fatal("terminal drop silently ignored instead of reaching upload validation")
	}
}

func TestDropBounds(t *testing.T) {
	origin := fyne.NewPos(200, 500)
	size := fyne.NewSize(800, 300)
	for _, tc := range []struct {
		pos  fyne.Position
		want bool
	}{{fyne.NewPos(200, 500), true}, {fyne.NewPos(600, 650), true}, {fyne.NewPos(199, 600), false}, {fyne.NewPos(600, 499), false}, {fyne.NewPos(1000, 600), false}, {fyne.NewPos(600, 800), false}} {
		if containsPosition(tc.pos, origin, size) != tc.want {
			t.Fatal(tc)
		}
	}
}
