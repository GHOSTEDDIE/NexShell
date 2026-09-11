//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer/zmodem"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"
)

type secretMap map[string]string

func (s secretMap) Get(id string) (string, error) {
	v, ok := s[id]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return v, nil
}
func TestLinuxLab(t *testing.T) {
	password := domain.ID()
	out, e := exec.Command("docker", "run", "--rm", "-d", "-p", "127.0.0.1::22", "-e", "TEST_PASSWORD="+password, "nexshell-test-sshd:local").CombinedOutput()
	if e != nil {
		t.Fatalf("start fixture: %s: %v", out, e)
	}
	cid := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		if b, e := exec.Command("docker", "rm", "-f", cid).CombinedOutput(); e != nil {
			t.Logf("fixture cleanup: %s %v", b, e)
		}
	})
	out, e = exec.Command("docker", "port", cid, "22/tcp").Output()
	if e != nil {
		t.Fatal(e)
	}
	address := strings.TrimSpace(string(out))
	host, portText, e := net.SplitHostPort(address)
	if e != nil {
		t.Fatal(e)
	}
	port, _ := strconv.Atoi(portText)
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := domain.Host{ID: "lab", Name: "Isolated Linux", Address: host, Port: port, User: "root", Auth: "password"}
	s.Put("hosts", h.ID, h)
	secrets := secretMap{"host:lab": password, "host:bad": "incorrect"}
	m, e := remote.NewManager(s, secrets, s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	timeout := 90 * time.Second
	if os.Getenv("NEXSHELL_REAL_MODEL") == "1" {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var key *remote.HostKeyError
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		_, e = m.Connect(ctx, h.ID)
		if errors.As(e, &key) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if key == nil {
		t.Fatalf("first-use trust was not required: %v", e)
	}
	keys, e := exec.Command("docker", "exec", cid, "sh", "-c", "for key in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf \"$key\" -E sha256; done").Output()
	if e != nil || !strings.Contains(string(keys), key.Fingerprint) {
		t.Fatalf("host identity does not match fixture: %v", e)
	}
	if e = m.Trust(key); e != nil {
		t.Fatal(e)
	}
	client, e := m.Connect(ctx, h.ID)
	if e != nil {
		t.Fatal(e)
	}
	t.Run("authentication_and_shell", func(t *testing.T) {
		bad := h
		bad.ID = "bad"
		s.Put("hosts", bad.ID, bad)
		if _, e = m.Connect(ctx, bad.ID); e == nil {
			t.Fatal("wrong password accepted")
		}
		var b bytes.Buffer
		literal := "quote' $(touch /tmp/SHOULD_NOT_EXIST)\n中文"
		code, e := m.Command(ctx, h.ID, "printf '%s' "+remote.Quote(literal), &b, &b)
		if e != nil || code != 0 || b.String() != literal {
			t.Fatalf("quoting mismatch %q %v", b.String(), e)
		}
	})
	t.Run("sftp_conflict_resume", func(t *testing.T) {
		original, hash, e := m.ReadFile(ctx, h.ID, "/tmp/fixture/healthy.txt")
		if e != nil {
			t.Fatal(e)
		}
		backup, e := m.WriteFile(ctx, h.ID, "/tmp/fixture/healthy.txt", hash, []byte("temporary change"))
		if e != nil || backup == "" {
			t.Fatal(backup, e)
		}
		if _, e = m.WriteFile(ctx, h.ID, "/tmp/fixture/healthy.txt", hash, []byte("stale overwrite")); e == nil {
			t.Fatal("stale update accepted")
		}
		_, newHash, _ := m.ReadFile(ctx, h.ID, "/tmp/fixture/healthy.txt")
		if _, e = m.WriteFile(ctx, h.ID, "/tmp/fixture/healthy.txt", newHash, original); e != nil {
			t.Fatal(e)
		}
		data := bytes.Repeat([]byte("健康\x00\xff"), 2000)
		local := filepath.Join(t.TempDir(), "payload")
		os.WriteFile(local, data, 0600)
		if _, e = m.WriteFile(ctx, h.ID, "/tmp/fixture/payload", "new", data[:200]); e != nil {
			t.Fatal(e)
		}
		if e = m.Transfer(ctx, h.ID, local, "/tmp/fixture/payload", true, true, nil); e != nil {
			t.Fatal(e)
		}
		dest := filepath.Join(t.TempDir(), "received")
		if e = m.Transfer(ctx, h.ID, dest, "/tmp/fixture/payload", false, false, nil); e != nil {
			t.Fatal(e)
		}
		got, _ := os.ReadFile(dest)
		if !bytes.Equal(got, data) {
			t.Fatal("file mismatch")
		}
		os.WriteFile(local, []byte("different prefix"), 0600)
		if e = m.Transfer(ctx, h.ID, local, "/tmp/fixture/payload", true, true, nil); e == nil {
			t.Fatal("incompatible resume accepted")
		}
	})
	t.Run("changed_host_key", func(t *testing.T) {
		p := filepath.Join(s.Dir, "known_hosts")
		original, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		defer os.WriteFile(p, original, 0600)
		pub, _, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		key, e := ssh.NewPublicKey(pub)
		if e != nil {
			t.Fatal(e)
		}
		os.WriteFile(p, []byte(knownhosts.Line([]string{address}, key)+"\n"), 0600)
		other, e := remote.NewManager(s, secrets, s.Dir)
		if e != nil {
			t.Fatal(e)
		}
		defer other.Close()
		_, e = other.Connect(ctx, h.ID)
		var changed *remote.HostKeyError
		if !errors.As(e, &changed) || !changed.Changed {
			t.Fatalf("changed identity not blocked: %v", e)
		}
		if e = other.Trust(changed); e == nil {
			t.Fatal("changed host was accepted as first use")
		}
	})
	t.Run("jump_host", func(t *testing.T) {
		jumped := h
		jumped.ID = "jumped"
		jumped.Address = "127.0.0.1"
		jumped.Port = 22
		jumped.JumpID = h.ID
		s.Put("hosts", jumped.ID, jumped)
		secrets["host:jumped"] = password
		_, e := m.Connect(ctx, jumped.ID)
		var first *remote.HostKeyError
		if errors.As(e, &first) {
			if e = m.Trust(first); e != nil {
				t.Fatal(e)
			}
		} else if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if code, e := m.Command(ctx, jumped.ID, "printf jump-ok", &b, &b); e != nil || code != 0 || b.String() != "jump-ok" {
			t.Fatal(code, b.String(), e)
		}
	})
	t.Run("local_package_upload", func(t *testing.T) { testLocalPackageUpload(t, ctx, s, m, h) })
	t.Run("local_package_agent", func(t *testing.T) { exerciseLocalDeployment(t, ctx, s, m, h, secrets, false) })
	t.Run("local_package_real_model", func(t *testing.T) {
		if os.Getenv("NEXSHELL_REAL_MODEL") != "1" {
			t.Skip("real model opt-in required")
		}
		exerciseLocalDeployment(t, ctx, s, m, h, secrets, true)
	})
	t.Run("cancel_unknown_not_replayed", func(t *testing.T) {
		executor := &remote.Executor{Manager: m, Store: s}
		short, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cancel()
		request := domain.Request{TaskID: "cancel-test", CallID: "once", HostID: h.ID, Operation: "shell", Command: "echo once >> /tmp/fixture/cancel-count; trap '' TERM; sleep 10"}
		started := time.Now()
		result, e := executor.Execute(short, request)
		if time.Since(started) > 4*time.Second {
			t.Fatal("cancellation blocked on remote process exit")
		}
		if e != nil {
			t.Fatal(e)
		}
		if result.Status != "unknown" {
			t.Fatalf("unconfirmed termination represented as %s", result.Status)
		}
		again, e := executor.Execute(ctx, request)
		if e != nil || again.ID != result.ID {
			t.Fatal("request replayed", e)
		}
		b, _, e := m.ReadFile(ctx, h.ID, "/tmp/fixture/cancel-count")
		if e != nil || string(b) != "once\n" {
			t.Fatal("mutation repeated", string(b), e)
		}
	})
	t.Run("monitor_and_forwarding", func(t *testing.T) {
		first, e := m.Monitor(ctx, h.ID, nil)
		if e != nil {
			t.Fatal(e)
		}
		second, e := m.Monitor(ctx, h.ID, &first)
		if e != nil || second.Memory.State != "ok" || second.CPU.State != "ok" {
			t.Fatalf("metrics: %+v %v", second, e)
		}
		processes, err := remote.ParseProcesses(fmt.Sprint(second.Processes.Value))
		if second.Processes.State != "ok" || err != nil || len(processes) == 0 {
			t.Fatalf("structured processes: %v %v", processes, err)
		}
		ports, err := remote.ParsePorts(fmt.Sprint(second.Ports.Value))
		if second.Ports.State != "ok" || err != nil || len(ports) == 0 {
			t.Fatalf("structured ports: %v %v", ports, err)
		}
		for _, kind := range []string{"local", "dynamic"} {
			tunnel, e := m.Tunnel(ctx, h.ID, kind, "127.0.0.1:0", "127.0.0.1:22")
			if e != nil {
				t.Fatal(e)
			}
			defer tunnel.Close()
			var conn net.Conn
			if kind == "local" {
				conn, e = net.DialTimeout("tcp", tunnel.Address, time.Second)
			} else {
				d, err := proxy.SOCKS5("tcp", tunnel.Address, nil, &net.Dialer{Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				conn, e = d.Dial("tcp", "127.0.0.1:22")
			}
			if e != nil {
				t.Fatal(e)
			}
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			b := make([]byte, 128)
			n, e := conn.Read(b)
			conn.Close()
			if e != nil || !strings.HasPrefix(string(b[:n]), "SSH-2.0-") {
				t.Fatalf("forwarding %s: %q %v", kind, b[:n], e)
			}
		}
	})
	t.Run("zmodem_lrzsz", func(t *testing.T) {
		data := bytes.Repeat([]byte{0, 1, 0x11, 0x13, 0x18, 0x7f, 0xff, 'x'}, 5000)
		dir := t.TempDir()
		names := []string{filepath.Join(dir, "binary.bin"), filepath.Join(dir, "empty")}
		os.WriteFile(names[0], data, 0600)
		os.WriteFile(names[1], nil, 0600)
		run := func(command string, fn func(*zmodem.Codec) error) {
			session, e := client.NewSession()
			if e != nil {
				t.Fatal(e)
			}
			defer session.Close()
			in, _ := session.StdinPipe()
			reader, _ := session.StdoutPipe()
			session.Stderr = io.Discard
			if e = session.RequestPty("xterm", 24, 80, ssh.TerminalModes{ssh.ECHO: 0, ssh.ICANON: 0, ssh.ISIG: 0, ssh.OPOST: 0}); e != nil {
				t.Fatal(e)
			}
			timeout := time.AfterFunc(15*time.Second, func() { session.Close() })
			defer timeout.Stop()
			if e = session.Start(command); e != nil {
				t.Fatal(e)
			}
			var wire bytes.Buffer
			if e = fn(zmodem.New(io.TeeReader(reader, &wire), in)); e != nil {
				data := wire.Bytes()
				if len(data) > 1024 {
					data = data[len(data)-1024:]
				}
				t.Fatalf("%s: %v; received=%q", command, e, data)
			}
			if e = session.Wait(); e != nil {
				t.Fatal(e)
			}
		}
		run("cd /tmp/fixture && rz -b -y", func(c *zmodem.Codec) error { return c.Send(ctx, names, nil, nil) })
		got, _, e := m.ReadFile(ctx, h.ID, "/tmp/fixture/binary.bin")
		if e != nil || !bytes.Equal(got, data) {
			t.Fatal("rz received wrong contents", e)
		}
		dest := t.TempDir()
		run("cd /tmp/fixture && sz -b -e binary.bin empty", func(c *zmodem.Codec) error { return c.Receive(ctx, dest, nil, nil) })
		got, e = os.ReadFile(filepath.Join(dest, "binary.bin"))
		if e != nil || !bytes.Equal(got, data) {
			t.Fatal("sz sent wrong contents", e)
		}
		resumeDir := t.TempDir()
		if e = os.WriteFile(filepath.Join(resumeDir, "binary.bin"), data[:1234], 0600); e != nil {
			t.Fatal(e)
		}
		run("cd /tmp/fixture && sz -b -e binary.bin", func(c *zmodem.Codec) error { return c.Receive(ctx, resumeDir, nil, nil) })
		got, e = os.ReadFile(filepath.Join(resumeDir, "binary.bin"))
		if e != nil || !bytes.Equal(got, data) {
			t.Fatal("ZMODEM resumed content differs", e)
		}
	})

	t.Run("companion_existing_terminal", func(t *testing.T) {
		session, err := m.Terminal(ctx, h.ID, 24, 80)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		initialDir, err := m.TerminalDirectory(ctx, h.ID, session)
		if err != nil || initialDir != "/root" {
			t.Fatal("initial terminal directory", initialDir, err)
		}

		core := terminal.NewCore(80, 24)
		defer core.CloseInput()
		go io.Copy(io.Discard, core)
		go io.Copy(core, session.Output)
		desktop := &remote.DesktopTerminals{}
		id := desktop.Register(h.ID, session.Input, func() string { return strings.Join(core.Lines(), "\n") }, func() bool { return true })
		ex := &remote.Executor{Manager: m, Store: s, Desktop: desktop}
		request := domain.Request{TaskID: "companion", CallID: "write", HostID: h.ID, Operation: "terminal_write", Resource: id, Content: "cd /tmp && export NEXSHELL_COMPANION=ready\n"}
		if r, err := ex.Execute(ctx, request); err != nil || r.Status != "succeeded" {
			t.Fatal(r, err)
		}
		request.CallID = "check"
		request.Content = "printf '%s:%s\\n' \"$NEXSHELL_COMPANION\" \"$PWD\"\n"
		if r, err := ex.Execute(ctx, request); err != nil || r.Status != "succeeded" {
			t.Fatal(r, err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			out, err := desktop.Perform(ctx, domain.Request{HostID: h.ID, Operation: "terminal_read", Resource: id})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "ready:/tmp") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("same-shell environment and working directory not preserved", out)
			}
			time.Sleep(20 * time.Millisecond)
		}
		currentDir, err := m.TerminalDirectory(ctx, h.ID, session)
		if err != nil || currentDir != "/tmp" {
			t.Fatal("directory did not follow cd", currentDir, err)
		}
		local := filepath.Join(t.TempDir(), "drop.txt")
		os.WriteFile(local, []byte("drop upload baseline"), 0600)
		if err := m.UploadNew(ctx, h.ID, local, "/tmp/drop.txt", nil); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(local, []byte("must not overwrite"), 0600)
		if err := m.UploadNew(ctx, h.ID, local, "/tmp/drop.txt", nil); err == nil {
			t.Fatal("exclusive upload overwrote existing file")
		}
		content, _, err := m.ReadFile(ctx, h.ID, "/tmp/drop.txt")
		if err != nil || string(content) != "drop upload baseline" {
			t.Fatal("upload baseline changed", err)
		}

	})
	t.Run("deep_terminal_delegation", func(t *testing.T) { exerciseDeepTerminal(t, ctx, m, s, secrets, false) })
	if os.Getenv("NEXSHELL_REAL_MODEL") == "1" {
		t.Run("deep_real_model", func(t *testing.T) { exerciseDeepTerminal(t, ctx, m, s, secrets, true) })
	}
	t.Run("eino_approval_execution_verification", func(t *testing.T) {
		if _, e := m.WriteFile(ctx, h.ID, "/tmp/fixture/repair.conf", "new", []byte("enabled=false\n")); e != nil {
			t.Fatal(e)
		}
		profile := domain.ModelProfile{ID: "test-model", Provider: "openai", Model: "deterministic", ContextTokens: 32000}
		s.Put("models", profile.ID, profile)
		ex := &remote.Executor{Manager: m, Store: s}
		service := agent.NewService(s, ex, secrets)
		defer service.Close()
		fake := &scenarioModel{}
		service.ModelFactory = func(context.Context, domain.ModelProfile, remote.Secrets) (model.ToolCallingChatModel, error) {
			return fake, nil
		}
		task, e := service.NewTask("将受控配置 enabled 改为 true，并验证健康基线不变", profile.ID, []string{h.ID}, []string{"observe", "file_read", "file_write"}, []string{"/tmp/fixture/repair.conf"})
		if e != nil {
			t.Fatal(e)
		}
		if e = service.Submit(task.ID, task.Goal); e != nil {
			t.Fatal(e)
		}
		await := func(want string) {
			t.Helper()
			for deadline := time.Now().Add(12 * time.Second); time.Now().Before(deadline); {
				s.Load("tasks", task.ID, &task)
				if task.Status == want {
					return
				}
				if task.Status == "failed" {
					t.Fatalf("task failed: %s", task.Summary)
				}
				time.Sleep(25 * time.Millisecond)
			}
			events, _ := s.Events(task.ID, 0)
			t.Fatalf("expected %s got %s (%s); events=%+v", want, task.Status, task.Summary, events)
		}
		await("awaiting_approval")
		results, _ := s.Results(task.ID)
		if len(results) != 0 {
			t.Fatal("shell executed without approval")
		}
		allApprovals, _ := store.All[agent.Approval](s, "approvals")
		approvals := []agent.Approval{}
		for _, a := range allApprovals {
			if a.Request.TaskID == task.ID {
				approvals = append(approvals, a)
			}
		}
		if len(approvals) != 1 {
			t.Fatalf("approval count %d", len(approvals))
		}
		if e = service.Decide(task.ID, approvals[0].Digest, true); e != nil {
			t.Fatal(e)
		}
		await("completed")
		results, _ = s.Results(task.ID)
		if len(results) != 4 {
			t.Fatalf("unexpected execution count %d", len(results))
		}
		for _, r := range results {
			if r.Status != "succeeded" {
				t.Fatal(r)
			}
		}
		changed, _, e := m.ReadFile(ctx, h.ID, "/tmp/fixture/repair.conf")
		if e != nil || string(changed) != "enabled=true\n" {
			t.Fatal("agent did not apply the real configuration change", e)
		}
		healthy, _, e := m.ReadFile(ctx, h.ID, "/tmp/fixture/healthy.txt")
		if e != nil || string(healthy) != "healthy baseline\n" {
			t.Fatal("known good baseline changed", e)
		}
	})
}

type scenarioModel struct {
	mu    sync.Mutex
	stage int
}

func (m *scenarioModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *scenarioModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stage := m.stage
	m.stage++
	call := func(id, name string, args any) *schema.Message {
		b, _ := json.Marshal(args)
		return schema.AssistantMessage("", []schema.ToolCall{{ID: id, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: string(b)}}})
	}
	switch stage {
	case 0:
		return call("shell-1", "operate", agent.OperationInput{HostID: "lab", Operation: "shell", Command: "printf 'approved execution'"}), nil
	case 1:
		return call("read-1", "operate", agent.OperationInput{HostID: "lab", Operation: "file_read", Resource: "/tmp/fixture/repair.conf"}), nil
	case 2:
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == schema.Tool {
				var result domain.Result
				var file map[string]string
				if e := json.Unmarshal([]byte(messages[i].Content), &result); e != nil {
					return nil, e
				}
				if e := json.Unmarshal([]byte(result.Output), &file); e != nil {
					return nil, e
				}
				return call("write-1", "operate", agent.OperationInput{HostID: "lab", Operation: "file_write", Resource: "/tmp/fixture/repair.conf", Content: "enabled=true\n", ExpectedHash: file["sha256"]}), nil
			}
		}
		return nil, errors.New("missing original file hash")
	case 3:
		return call("read-2", "operate", agent.OperationInput{HostID: "lab", Operation: "file_read", Resource: "/tmp/fixture/repair.conf"}), nil
	case 4:
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == schema.Tool {
				var r domain.Result
				if e := json.Unmarshal([]byte(messages[i].Content), &r); e != nil {
					return nil, fmt.Errorf("parse tool result: %w", e)
				}
				return call("verify-1", "verify_execution", agent.VerifyInput{ExecutionID: r.ID, ExpectedText: "enabled=true"}), nil
			}
		}
		return nil, errors.New("missing evidence")
	default:
		return schema.AssistantMessage("健康基线已验证。", nil), nil
	}
}
func (m *scenarioModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, e := m.Generate(ctx, messages, opts...)
	if e != nil {
		return nil, e
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}
