package main

import (
	_ "embed"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/ui"
	"os"
	"path/filepath"
)

//go:embed assets/nexshell-icon.png
var appIcon []byte

func main() {
	data := flag.String("data-dir", "", "本地配置与任务目录")
	flag.Parse()
	if *data == "" {
		root, e := os.UserConfigDir()
		if e != nil {
			fatal(e)
		}
		*data = filepath.Join(root, "NexShell")
	}
	s, e := store.Open(*data)
	if e != nil {
		fatal(e)
	}
	if e = s.Recover(); e != nil {
		fatal(e)
	}
	secrets := store.Credentials{}
	m, e := remote.NewManager(s, secrets, *data)
	if e != nil {
		fatal(e)
	}
	executor := &remote.Executor{Manager: m, Store: s}
	agents := agent.NewService(s, executor, secrets)
	a := app.NewWithID("io.nexshell.desktop")
	a.SetIcon(fyne.NewStaticResource("nexshell-icon.png", appIcon))
	ui.New(a, s, m, executor, agents).Show()
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
