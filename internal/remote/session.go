package remote

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"io"
	"strings"
	"sync"
	"time"
)

func replaceQuotes(s string) string { return strings.ReplaceAll(s, "'", "'\"'\"'") }

type TerminalSession struct {
	Session *ssh.Session
	Input   io.WriteCloser
	Output  io.Reader
	Done    chan error
	once    sync.Once
}
type serialWriter struct {
	gate   *outputGate
	target io.Writer
}
type outputGate struct {
	mu     sync.Mutex
	closed bool
}

func (w serialWriter) Write(b []byte) (int, error) {
	w.gate.mu.Lock()
	defer w.gate.mu.Unlock()
	if w.gate.closed {
		return len(b), nil
	}
	return w.target.Write(b)
}

func (m *Manager) Terminal(ctx context.Context, host string, rows, cols int) (*TerminalSession, error) {
	c, err := m.Connect(ctx, host)
	if err != nil {
		return nil, err
	}
	s, err := c.NewSession()
	if err != nil {
		return nil, err
	}
	in, err := s.StdinPipe()
	if err != nil {
		s.Close()
		return nil, err
	}
	out, err := s.StdoutPipe()
	if err != nil {
		s.Close()
		return nil, err
	}
	if err = s.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}); err != nil {
		s.Close()
		return nil, err
	}
	if err = s.Shell(); err != nil {
		s.Close()
		return nil, err
	}
	t := &TerminalSession{Session: s, Input: in, Output: out, Done: make(chan error, 1)}
	go func() { t.Done <- s.Wait(); close(t.Done); t.Close() }()
	return t, nil
}
func (t *TerminalSession) Resize(rows, cols int) error {
	if rows < 1 || cols < 1 {
		return nil
	}
	return t.Session.WindowChange(rows, cols)
}
func (t *TerminalSession) Close() { t.once.Do(func() { t.Session.Close() }) }

// Command never infers a successful remote cancellation from a closed SSH channel.
func (m *Manager) Command(ctx context.Context, host, command string, stdout, stderr io.Writer) (int, error) {
	c, err := m.Connect(ctx, host)
	if err != nil {
		return -1, err
	}
	s, err := c.NewSession()
	if err != nil {
		return -1, err
	}
	defer s.Close()
	gate := &outputGate{}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	s.Stdout = serialWriter{gate, stdout}
	s.Stderr = serialWriter{gate, stderr}
	if err = s.Start(command); err != nil {
		return -1, err
	}
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	select {
	case err = <-done:
		return exitStatus(err)
	case <-ctx.Done():
		// Prefer a concurrently available exit over cancellation.
		select {
		case err = <-done:
			return exitStatus(err)
		default:
		}
		_ = s.Signal(ssh.SIGTERM)
		select {
		case err = <-done:
			return exitStatus(err)
		case <-time.After(2 * time.Second):
			// Freeze the caller's buffers before returning an unknown outcome.
			// SSH Close does not promise the peer's process has exited.
			gate.mu.Lock()
			gate.closed = true
			gate.mu.Unlock()
			s.Close()
			return -1, fmtUnknown(ctx.Err())
		}
	}
}
func exitStatus(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var ee *ssh.ExitError
	if errors.As(err, &ee) {
		return ee.ExitStatus(), err
	}
	return -1, err
}
func fmtUnknown(err error) error { return errors.New("远端执行结果尚未确认: " + err.Error()) }
