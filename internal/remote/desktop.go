package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"io"
	"sync"
)

// DesktopTerminals identifies sessions independently of the selected tab.
// Snapshots and writes are supplied by the same terminal view used by the user.
type DesktopTerminals struct {
	mu       sync.Mutex
	sessions map[string]desktopTerminal
	active   string
	Open     func(context.Context, string) (string, error)
}
type desktopTerminal struct {
	host     string
	input    io.Writer
	snapshot func() string
	alive    func() bool
}
type TerminalInfo struct {
	ID     string `json:"terminal_id"`
	HostID string `json:"host_id"`
	Active bool   `json:"active"`
}

func (d *DesktopTerminals) Register(host string, input io.Writer, snapshot func() string, alive func() bool) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sessions == nil {
		d.sessions = map[string]desktopTerminal{}
	}
	id := domain.ID()
	d.sessions[id] = desktopTerminal{host, input, snapshot, alive}
	return id
}
func (d *DesktopTerminals) Remove(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sessions, id)
	if d.active == id {
		d.active = ""
	}
}
func (d *DesktopTerminals) Select(id string) { d.mu.Lock(); defer d.mu.Unlock(); d.active = id }
func (d *DesktopTerminals) List(hosts []string) []TerminalInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []TerminalInfo{}
	for id, s := range d.sessions {
		for _, h := range hosts {
			if h == s.host && s.alive() {
				out = append(out, TerminalInfo{id, h, id == d.active})
				break
			}
		}
	}
	return out
}
func (d *DesktopTerminals) Perform(ctx context.Context, r domain.Request) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.Operation == "terminal_connect" {
		if d.Open == nil {
			return "", errors.New("终端工作台不可用")
		}
		id, err := d.Open(ctx, r.HostID)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(TerminalInfo{ID: id, HostID: r.HostID})
		return string(b), nil
	}
	d.mu.Lock()
	s, ok := d.sessions[r.Resource]
	d.mu.Unlock()
	if !ok || s.host != r.HostID || !s.alive() {
		return "", errors.New("指定终端已关闭或不属于目标服务器")
	}
	switch r.Operation {
	case "terminal_read":
		text := []rune(s.snapshot())
		if len(text) > 16000 {
			text = text[len(text)-16000:]
		}
		return string(text), nil
	case "terminal_write":
		if r.Content == "" || len(r.Content) > 16384 {
			return "", errors.New("终端输入必须为 1 至 16384 字节")
		}
		done := make(chan error, 1)
		go func() {
			n, err := s.input.Write([]byte(r.Content))
			if err == nil && n != len(r.Content) {
				err = io.ErrShortWrite
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				return "", err
			}
			return "输入已发送到指定终端；这不代表命令完成或成功。请读取终端输出并独立验证结果。", nil
		case <-ctx.Done():
			return "", fmt.Errorf("终端输入交付结果未知: %w", ctx.Err())
		}
	}
	return "", errors.New("未知终端操作")
}
