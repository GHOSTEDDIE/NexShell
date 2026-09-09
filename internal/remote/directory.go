package remote

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"io"
	"strconv"
	"strings"
)

// Start the account's login shell with a stable PID. The private startup marker is
// consumed before terminal rendering; subsequent output is never parsed for paths.
func terminalStartup() (command, marker string) {
	marker = "\x1eNexShell:" + domain.ID() + ":"
	script := "printf " + Quote(marker+"%s\x1f") + " \"$$\"; exec \"$SHELL\" -l"
	return "exec /bin/sh -c " + Quote(script), marker
}
func readTerminalPID(r io.Reader, marker string) (int, io.Reader, error) {
	reader := bufio.NewReader(r)
	prefix := []byte{}
	for len(prefix) < 65536 {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		prefix = append(prefix, b)
		if b != 0x1f {
			continue
		}
		start := bytes.Index(prefix, []byte(marker))
		if start < 0 {
			continue
		}
		pid, err := strconv.Atoi(string(prefix[start+len(marker) : len(prefix)-1]))
		if err != nil || pid <= 0 {
			return 0, nil, fmt.Errorf("无法识别终端进程")
		}
		return pid, io.MultiReader(bytes.NewReader(prefix[:start]), reader), nil
	}
	return 0, nil, fmt.Errorf("终端启动输出超过限制")
}
func (m *Manager) TerminalDirectory(ctx context.Context, host string, t *TerminalSession) (string, error) {
	select {
	case <-t.closed:
		return "", fmt.Errorf("终端已关闭")
	default:
	}
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return "", err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	dir, err := c.ReadLink(fmt.Sprintf("/proc/%d/cwd", t.PID))
	if err != nil {
		return "", fmt.Errorf("读取终端目录失败: %w", err)
	}
	if !strings.HasPrefix(dir, "/") {
		return "", fmt.Errorf("终端目录不是绝对路径")
	}
	select {
	case <-t.closed:
		return "", fmt.Errorf("终端已关闭")
	default:
	}
	return dir, nil
}
