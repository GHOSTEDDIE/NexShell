//go:build darwin && lifecycle_probe

// A separate application identity and temporary store keep native lifecycle
// checks away from saved connections and the user's running NexShell instance.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2/app"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/ui"
)

func main() {
	dir := filepath.Join(os.TempDir(), "nexshell-lifecycle-probe")
	s, err := store.Open(dir)
	if err != nil {
		panic(err)
	}
	m, err := remote.NewManager(s, store.Credentials{}, s.Dir)
	if err != nil {
		panic(err)
	}
	e := &remote.Executor{Store: s, Manager: m}
	agents := agent.NewService(s, e, store.Credentials{})
	a := app.NewWithID("io.nexshell.lifecycle.probe")
	u := ui.New(a, s, m, e, agents)
	u.Window.SetTitle("NexShell Lifecycle Probe")
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := s.Event("lifecycle-probe", "heartbeat", "alive"); err != nil {
				return
			}
		}
	}()
	u.Show()
	_ = os.WriteFile(filepath.Join(dir, "exited"), []byte(fmt.Sprint(time.Now().UnixNano())), 0600)
}
