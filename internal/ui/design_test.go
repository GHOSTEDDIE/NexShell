package ui

import (
	"context"
	"errors"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func referenceSnapshot() remote.Snapshot {
	return remote.Snapshot{At: time.Date(2026, 9, 9, 14, 32, 8, 0, time.Local), CPU: remote.Metric{State: "ok", Value: 8.2}, Memory: remote.Metric{State: "ok", Value: map[string]uint64{"total": 16 << 30, "used": uint64(16<<30) * 425 / 1000}}, Load: remote.Metric{State: "ok", Value: "0.12 0.18 0.15 1/100 1234"}, Disks: remote.Metric{State: "ok", Value: "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vdb1 209715200 134217728 75497472 64% /data\n/dev/vda1 41943040 11744051 30198989 28% /"}, Network: remote.Metric{State: "ok", Value: map[string]remote.NetworkRate{"eth0": {ReceiveBytesPerSecond: 128400, SendBytesPerSecond: 32100}}}, System: remote.Metric{State: "ok", Value: "Ubuntu 24.04"}, Uptime: remote.Metric{State: "ok", Value: "1047600.00 0"}, Processes: remote.Metric{State: "ok", Value: "PID USER %CPU %MEM COMMAND\n 12 root 0.3 1.0 nginx"}, Ports: remote.Metric{State: "ok", Value: "tcp LISTEN 0 128 *:22 *:* users:((\"sshd\",pid=834,fd=3))"}}
}
func TestServerMetricsUsesSnapshotAndClearsStaleValues(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	m := newServerMetrics()
	s := referenceSnapshot()
	m.apply(s, nil)
	if m.cpu.value.Text != "8.2%" || m.memory.value.Text != "42.5%" || len(m.disks.Objects) != 4 {
		t.Fatal("valid monitor presentation regressed", m.cpu.value.Text, m.memory.value.Text, len(m.disks.Objects))
	}
	m.apply(s, errors.New("offline"))
	if m.cpu.bar.value != 0 || m.memory.bar.value != 0 || m.lastDisk != "" || m.system.Text != "数据已过期" {
		t.Fatal("stale status remains healthy")
	}
	m.apply(s, nil)
	if m.cpu.value.Text != "8.2%" || m.lastDisk == "" {
		t.Fatal("monitor did not recover")
	}
}
func TestSystemAppearanceAndTextScale(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	setAppearanceMode(a, "system")
	th := a.Settings().Theme()
	if th.Color(theme.ColorNameBackground, theme.VariantLight) == th.Color(theme.ColorNameBackground, theme.VariantDark) {
		t.Fatal("system variant ignored")
	}
	setAppearanceMode(a, "dark")
	th = a.Settings().Theme()
	if th.Color(theme.ColorNameBackground, theme.VariantLight) != th.Color(theme.ColorNameBackground, theme.VariantDark) {
		t.Fatal("manual dark theme follows OS")
	}
	a.Preferences().SetFloat("appearance.textScale", 15.0/14)
	restoreTheme(a)
	if a.Settings().Theme().Size(sizeBody) != 15 {
		t.Fatal("font scale not restored")
	}
	if th.Font(fyne.TextStyle{}) == th.Font(fyne.TextStyle{Monospace: true}) || th.Font(fyne.TextStyle{}) == th.Font(fyne.TextStyle{Bold: true}) {
		t.Fatal("proportional and weighted fonts missing")
	}
}
func TestChatMessagesKeepExecutionSeparate(t *testing.T) {
	var m []chatMessage
	for _, e := range []domain.Event{{Kind: "user", Text: "检查\n\n[发送时的当前终端：host_id=A]"}, {Kind: "assistant_delta", Text: "运行"}, {Kind: "assistant_delta", Text: "正常"}, {Kind: "execution_start", Text: "uptime"}, {Kind: "execution_result", Text: "ok"}, {Kind: "assistant", Text: "完成"}} {
		m = appendChatEvent(m, e)
	}
	if len(m) != 4 || m[0].text != "检查" || m[1].text != "运行正常" || m[2].kind != "execution" || m[3].text != "完成" {
		t.Fatalf("messages corrupted: %+v", m)
	}
}

func TestPrototypeReferenceScene(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	setAppearance(a, false)
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	manager, e := remote.NewManager(s, store.Credentials{}, s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer manager.Close()
	executor := &remote.Executor{Manager: manager, Store: s}
	ag := agent.NewService(s, executor, store.Credentials{})
	defer ag.Close()
	u := New(a, s, manager, executor, ag)
	defer func() { u.cancel(); u.Window.SetCloseIntercept(nil); u.Window.Close() }()
	hosts := []domain.Host{{ID: "a", Name: "生产应用 01", Address: "10.0.1.21", Port: 22, User: "root", Group: "生产环境"}, {ID: "b", Name: "生产应用 02", Address: "10.0.1.22", Port: 22, User: "root", Group: "生产环境"}, {ID: "db", Name: "生产数据库", Address: "10.0.1.30", Port: 22, User: "root", Group: "生产环境"}, {ID: "test", Name: "测试服务器", Address: "10.0.2.10", Port: 22, User: "root", Group: "测试环境"}}
	for _, h := range hosts {
		if e = s.Put("hosts", h.ID, h); e != nil {
			t.Fatal(e)
		}
	}
	u.refreshHosts()
	metrics := newServerMetrics(u.Window)
	snap := referenceSnapshot()
	snap.Processes.Value = "PID USER %CPU %MEM COMMAND\n1916 9987 0.5 0.8 ts3server\n834 root 250.5 20.0 node\n810903 root 0.2 0.4 sshd\n623077 root 0.2 1.0 service worker\n740 root 0.1 1.9 containerd"
	snap.Ports.Value = "Netid State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\ntcp LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:((\"sshd\",pid=834,fd=3))\ntcp LISTEN 0 4096 [::]:8080 [::]:* users:((\"node\",pid=1916,fd=4))\nudp UNCONN 0 0 127.0.0.53%lo:53 0.0.0.0:*"
	metrics.apply(snap, nil)
	view := terminal.NewView(io.Discard, strings.NewReader(""))
	view.Close()
	ctx, cancel := context.WithCancel(u.ctx)
	defer cancel()
	w := &workspace{u: u, host: hosts[0], ctx: ctx, cancel: cancel, terminal: view, terminals: []*terminal.View{view}, monitor: metrics.content, selectedFile: -1}
	w.layoutWorkspace()
	u.showWorkspace(w)
	w.dir.SetText("/var/log")
	w.files = []remote.FileEntry{{Name: "nginx", IsDir: true, Modified: time.Now()}, {Name: "journal", IsDir: true, Modified: time.Now()}, {Name: "syslog", Size: 2400000, Modified: time.Now()}, {Name: "auth.log", Size: 864000, Modified: time.Now()}}
	w.listedDirectory = "/var/log"
	w.fileList.Refresh()
	u.conversationView.update("reference", []chatMessage{{"user", "帮我检查这台服务器的运行状态"}, {"assistant", "已完成检查，服务器运行正常。"}, {"verified", "CPU 负载                  0.12\n运行中容器                 3 个\n数据盘使用率               64%"}, {"assistant", "暂未发现异常，磁盘空间充足。"}, {"execution", "uptime\ndocker ps\ndf -h /data"}})
	u.Window.Resize(fyne.NewSize(1440, 940))
	u.Window.Show()
	_, _ = view.Core.Write([]byte("\x1b[38;2;131;196;173mroot@app-01:~# uptime\x1b[0m\r\n14:32:08 up 12 days, 3:41, 2 users, load average: 0.12, 0.18, 0.15\r\n\r\n\x1b[38;2;131;196;173mroot@app-01:~# docker ps\x1b[0m\r\nNAMES          STATUS          PORTS\r\nnginx          Up 12 days      0.0.0.0:80->80/tcp\r\napp-server     Up 12 days      0.0.0.0:8080->8080/tcp\r\nredis          Up 12 days      6379/tcp\r\n\r\nroot@app-01:~# df -h /data\r\nFilesystem     Size   Used   Avail   Use%   Mounted on\r\n/dev/vdb1      200G   128G    72G    64%   /data\r\n\r\nroot@app-01:~# "))
	view.Refresh()
	if u.sideTabs.Selected().Text != "服务器状态" {
		t.Fatal("wrong initial sidebar")
	}
	u.sideTabs.SelectIndex(1)
	u.selectWorkspace(w.tab)
	if u.sideTabs.Selected().Text != "连接列表" {
		t.Fatal("session switch stole sidebar selection")
	}
	u.sideTabs.SelectIndex(0)
	save := func(name string) {
		t.Helper()
		dir := os.Getenv("MYAIT_DESIGN_EVIDENCE")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, u.Window.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	save("workbench-light")
	u.sideTabs.SelectIndex(1)
	save("connections-light")
	u.sideTabs.SelectIndex(0)
	setAppearance(a, true)
	save("workbench-dark")
	u.settingsDialog("models")
	save("settings-models-dark")
	u.settingsPopup.Hide()
	setAppearance(a, false)
	u.settingsDialog("appearance")
	save("settings-appearance-light")
	u.settingsPopup.Hide()
	metrics.details.Open(0)
	metrics.content.(*container.Scroll).ScrollToBottom()
	save("process-table")
	metrics.processes.showExpanded()
	save("process-all-columns")
	metrics.processes.expandedDialog.Hide()
	metrics.details.Close(0)
	metrics.details.Open(1)
	metrics.content.(*container.Scroll).ScrollToBottom()
	save("port-table")
	metrics.ports.showExpanded()
	save("port-all-columns")
	metrics.ports.expandedDialog.Hide()

	// Editing the current profile must update an already selected composer model.
	profile := domain.ModelProfile{ID: "model", Name: "默认模型", Model: "old-model", Provider: "openai", ContextTokens: 32000}
	if err := s.Put("models", profile.ID, profile); err != nil {
		t.Fatal(err)
	}
	u.refreshModelChoices()
	profile.Model = "updated-model"
	if err := s.Put("models", profile.ID, profile); err != nil {
		t.Fatal(err)
	}
	u.refreshModelChoices()
	if u.modelSelect.Selected != "默认模型 · updated-model" {
		t.Fatal("saved model label did not refresh")
	}
}
