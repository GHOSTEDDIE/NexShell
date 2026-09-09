package ui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"testing"
)

func TestConnectionRowSingleAndDoubleClick(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	row := newConnectionRow()
	selected, connected := 0, 0
	row.selectHost = func() { selected++ }
	row.connectHost = func() { connected++ }
	test.Tap(row)
	if selected != 1 || connected != 0 {
		t.Fatal("single click must only select")
	}
	test.DoubleTap(row)
	if selected != 2 || connected != 1 {
		t.Fatal("double click must connect exactly once")
	}
	// Virtualized rows can be rebound to another saved connection.
	newTarget := false
	row.connectHost = func() { newTarget = true }
	test.DoubleTap(row)
	if !newTarget || connected != 1 {
		t.Fatal("recycled row used previous connection")
	}
}
func TestServerStatusFollowsSelectedTab(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed, closeCtx := context.WithCancel(context.Background())
	a := &workspace{ctx: ctx, host: domain.Host{ID: "A", Name: "Server A"}, tab: container.NewTabItem("A", widget.NewLabel("terminal A")), monitor: widget.NewLabel("metrics A")}
	b := &workspace{ctx: closed, host: domain.Host{ID: "B", Name: "Server B"}, tab: container.NewTabItem("B", widget.NewLabel("terminal B")), monitor: widget.NewLabel("metrics B")}
	u := &App{terminalsDesktop: &remote.DesktopTerminals{}, serverStatus: container.NewStack(emptyServerStatus()), workspaces: []*workspace{a, b}}
	for _, w := range []*workspace{a, b, a} {
		u.selectWorkspace(w.tab)
		pane := u.serverStatus.Objects[0].(*fyne.Container)
		if pane.Objects[0] != w.monitor || u.selected != w.host.ID {
			t.Fatal("status does not belong to selected server")
		}
	}
	closeCtx()
	u.selectWorkspace(b.tab)
	if u.serverStatus.Objects[0].(*fyne.Container).Objects[0] == b.monitor {
		t.Fatal("closed workspace still visible")
	}
	u.selectWorkspace(container.NewTabItem("工作台", widget.NewLabel("home")))
	if u.serverStatus.Objects[0].(*fyne.Container).Objects[0] == a.monitor {
		t.Fatal("home shows previous server metrics")
	}
}
