//go:build ci && integration

package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
)

type dropSecrets string

func (s dropSecrets) Get(string) (string, error) { return string(s), nil }

func TestDroppedFilesUploadToIntendedDirectory(t *testing.T) {
	password := domain.ID()
	out, err := exec.Command("docker", "run", "--rm", "-d", "-p", "127.0.0.1::22", "-e", "TEST_PASSWORD="+password, "nexshell-test-sshd:local").CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	cid := strings.TrimSpace(string(out))
	defer exec.Command("docker", "rm", "-f", cid).Run()
	out, err = exec.Command("docker", "port", cid, "22/tcp").Output()
	if err != nil {
		t.Fatal(err)
	}
	address, portText, err := net.SplitHostPort(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	host := domain.Host{ID: "drop-fixture", Name: "Drop fixture", Address: address, Port: port, User: "root", Auth: "password"}
	if err = s.Put("hosts", host.ID, host); err != nil {
		t.Fatal(err)
	}
	manager, err := remote.NewManager(s, dropSecrets(password), s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var key *remote.HostKeyError
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); {
		_, err = manager.Connect(ctx, host.ID)
		if errors.As(err, &key) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if key == nil {
		t.Fatalf("fixture trust: %v", err)
	}
	// Check identity against the local container rather than accepting an arbitrary listener.
	out, err = exec.Command("docker", "exec", cid, "sh", "-c", "for key in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf \"$key\" -E sha256; done").Output()
	if err != nil || !strings.Contains(string(out), key.Fingerprint) {
		t.Fatal("fixture identity mismatch")
	}
	if err = manager.Trust(key); err != nil {
		t.Fatal(err)
	}
	sf, err := manager.SFTP(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer sf.Close()
	for _, dir := range []string{"/tmp/drop-left", "/tmp/drop-right", "/tmp/drop-browser"} {
		if err = sf.MkdirAll(dir); err != nil {
			t.Fatal(err)
		}
	}
	var sessions []*remote.TerminalSession
	for _, dir := range []string{"/tmp/drop-left", "/tmp/drop-right"} {
		session, err := manager.Terminal(ctx, host.ID, 24, 80)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		go io.Copy(io.Discard, session.Output)
		fmt.Fprintf(session.Input, "cd -- %s\r", remote.Quote(dir))
		for until := time.Now().Add(3 * time.Second); ; {
			actual, e := manager.TerminalDirectory(ctx, host.ID, session)
			if e == nil && actual == dir {
				break
			}
			if time.Now().After(until) {
				t.Fatalf("terminal directory: %s %v", actual, e)
			}
			time.Sleep(20 * time.Millisecond)
		}
		sessions = append(sessions, session)
	}
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	win := a.NewWindow("drop regression")
	defer win.Close()
	left := terminal.NewView(io.Discard, strings.NewReader(""))
	left.Close()
	right := terminal.NewView(io.Discard, strings.NewReader(""))
	right.Close()
	files := widget.NewLabel("files")
	fileTab := container.NewTabItem("文件", files)
	transfers := container.NewVBox()
	bottom := newTabView(false, fileTab, container.NewTabItem("传输", transfers))
	tab := container.NewTabItem("fixture", container.NewBorder(nil, bottom, nil, nil, container.NewGridWithColumns(2, left, right)))
	u := &App{UI: a, Window: win, tabs: newTabView(false, tab), Manager: manager, ctx: ctx, status: widget.NewLabel("")}
	w := &workspace{u: u, host: host, ctx: ctx, tab: tab, terminal: left, terminals: []*terminal.View{left, right}, sessions: sessions, fileArea: files, fileTab: fileTab, bottomTabs: bottom, dir: widget.NewEntry(), transfers: transfers, fileList: widget.NewList(func() int { return 0 }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(int, fyne.CanvasObject) {})}
	w.activeSession.Store(sessions[1])
	u.workspaces = []*workspace{w}
	win.SetContent(u.tabs)
	win.Resize(fyne.NewSize(900, 650))
	win.Show()
	bottom.Select(bottom.Items[1])
	local := filepath.Join(t.TempDir(), "中文 空格 100%.txt")
	content := []byte("actual dropped-file transfer\n")
	if err = os.WriteFile(local, content, 0600); err != nil {
		t.Fatal(err)
	}
	drop := func(area fyne.CanvasObject) {
		u.filesDropped(a.Driver().AbsolutePositionForObject(area).Add(fyne.NewPos(12, 12)), []fyne.URI{storage.NewFileURI(local)})
	}
	verify := func(dir string) {
		t.Helper()
		dest := path.Join(dir, filepath.Base(local))
		for until := time.Now().Add(5 * time.Second); ; {
			file, e := sf.Open(dest)
			if e == nil {
				data, _ := io.ReadAll(file)
				file.Close()
				if string(data) == string(content) {
					return
				}
			}
			if time.Now().After(until) {
				t.Fatalf("drop did not upload intact file to %s: %v", dest, e)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	drop(left)
	verify("/tmp/drop-left")
	if _, err = sf.Stat(path.Join("/tmp/drop-right", filepath.Base(local))); !os.IsNotExist(err) {
		t.Fatal("drop incorrectly followed keyboard-focused split")
	}
	// The file browser has its own destination. Capture it before navigation changes.
	bottom.Select(fileTab)
	w.dir.SetText("/tmp/drop-browser")
	drop(files)
	w.dir.SetText("")
	verify("/tmp/drop-browser")
	// A repeated drop must not silently overwrite an existing remote file.
	time.Sleep(100 * time.Millisecond)
	if err = os.WriteFile(local, []byte("changed local file"), 0600); err != nil {
		t.Fatal(err)
	}
	drop(left)
	for until := time.Now().Add(3 * time.Second); len(win.Canvas().Overlays().List()) == 0; {
		if time.Now().After(until) {
			t.Fatal("overwrite choice was not shown")
		}
		time.Sleep(20 * time.Millisecond)
	}
	verify("/tmp/drop-left")
	time.Sleep(100 * time.Millisecond)
}
