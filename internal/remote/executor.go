package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Executor struct {
	Desktop *DesktopTerminals
	Manager *Manager
	Store   *store.Store
	locks   sync.Map
}

var resourceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.@:+-]*$`)

func Mutates(op string) bool {
	return op == "upload_local" || op == "terminal_write" || op == "file_write" || op == "service_restart" || op == "package_install" || op == "shell"
}
func BuildCommand(r domain.Request) (string, error) {
	switch r.Operation {
	case "observe":
		return monitorScript, nil
	case "service_status", "service_restart":
		if !resourceName.MatchString(r.Resource) {
			return "", errors.New("无效的服务名")
		}
		verb := "status"
		if r.Operation == "service_restart" {
			verb = "restart"
		}
		return "systemctl --no-pager " + verb + " -- " + Quote(r.Resource), nil
	case "logs":
		if !resourceName.MatchString(r.Resource) {
			return "", errors.New("无效的服务名")
		}
		return "journalctl --no-pager -n 150 -u " + Quote(r.Resource), nil
	case "package_install":
		if !resourceName.MatchString(r.Resource) || strings.HasPrefix(r.Resource, "-") {
			return "", errors.New("无效的软件包名")
		}
		return "if command -v apt-get >/dev/null 2>&1; then DEBIAN_FRONTEND=noninteractive apt-get install -y -- " + Quote(r.Resource) + "; elif command -v dnf >/dev/null 2>&1; then dnf install -y -- " + Quote(r.Resource) + "; else printf '%s\\n' '不支持的软件包管理器' >&2; exit 127; fi", nil
	case "shell":
		if strings.TrimSpace(r.Command) == "" {
			return "", errors.New("命令为空")
		}
		return r.Command, nil
	default:
		return "", fmt.Errorf("不支持的操作: %s", r.Operation)
	}
}

type capture struct {
	mu      sync.Mutex
	out     io.Writer
	prefix  strings.Builder
	err     error
	onChunk func(string) error
}

func (c *capture) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, e := c.out.Write(b)
	if e != nil {
		c.err = e
	}
	remaining := 32768 - c.prefix.Len()
	if remaining > 0 {
		if len(b) < remaining {
			remaining = len(b)
		}
		c.prefix.Write(b[:remaining])
		if c.onChunk != nil {
			if err := c.onChunk(string(b[:remaining])); err != nil {
				c.err = err
				return n, err
			}
		}
	}
	return n, e
}
func (e *Executor) Execute(ctx context.Context, r domain.Request) (domain.Result, error) {
	if r.TaskID == "" || r.CallID == "" || (r.HostID == "" && !localfiles.IsOperation(r.Operation)) {
		return domain.Result{}, errors.New("缺少执行身份")
	}
	if Mutates(r.Operation) {
		v, _ := e.locks.LoadOrStore(r.HostID, make(chan struct{}, 1))
		lock := v.(chan struct{})
		select {
		case lock <- struct{}{}:
			defer func() { <-lock }()
		case <-ctx.Done():
			return domain.Result{}, ctx.Err()
		}
	}
	if err := checkAuthority(ctx); err != nil {
		return domain.Result{}, err
	}
	result, fresh, err := e.Store.Claim(r)
	if err != nil || !fresh {
		return result, err
	}
	artifact, f, err := e.Store.Artifact(result.ID)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		e.Store.Finish(result)
		return result, err
	}
	defer f.Close()
	result.Artifact = artifact
	if err = e.Store.Finish(result); err != nil {
		return result, err
	}
	cap := &capture{out: f, onChunk: func(chunk string) error { _, err := e.Store.Event(r.TaskID, "execution_delta", chunk); return err }}
	timeout := r.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	switch r.Operation {
	case "local_stat", "local_list", "local_read":
		var info localfiles.Info
		info, err = localfiles.Inspect(ctx, r.LocalPath, r.Operation, r.ReadOffset)
		if err == nil {
			var payload []byte
			payload, err = json.Marshal(info)
			if err == nil {
				_, err = cap.Write(payload)
			}
			result.ExitCode = 0
		}
	case "upload_local":
		var started bool
		started, err = e.uploadLocal(ctx, r, cap)
		if err == nil || !started {
			result.ExitCode = 0
		}
	case "file_hash":
		var hash string
		var size int64
		hash, size, err = e.Manager.FileHash(ctx, r.HostID, r.Resource)
		if err == nil {
			_, err = fmt.Fprintf(cap, "sha256=%s\nsize=%d", hash, size)
			result.ExitCode = 0
		}
	case "terminal_connect", "terminal_read", "terminal_write":
		if e.Desktop == nil {
			err = errors.New("终端工作台不可用")
		} else {
			var output string
			output, err = e.Desktop.Perform(ctx, r)
			if err == nil {
				_, err = cap.Write([]byte(output))
				result.ExitCode = 0
			}
		}
	case "file_read":
		var b []byte
		var hash string
		b, hash, err = e.Manager.ReadFile(ctx, r.HostID, r.Resource)
		if err == nil {
			payload, _ := json.Marshal(map[string]string{"content": string(b), "sha256": hash})
			_, err = cap.Write(payload)
		}
		if err == nil {
			result.ExitCode = 0
		}
	case "file_write":
		var backup string
		backup, err = e.Manager.WriteFile(ctx, r.HostID, r.Resource, r.ExpectedHash, []byte(r.Content))
		if err == nil {
			b, hash, verifyErr := e.Manager.ReadFile(ctx, r.HostID, r.Resource)
			if verifyErr != nil {
				err = verifyErr
			} else if string(b) != r.Content {
				err = errors.New("写入后校验不一致")
			} else {
				result.ExitCode = 0
				fmt.Fprintf(cap, "已写入并校验 sha256=%s 备份=%s", hash, backup)
			}
		}
	default:
		var cmd string
		cmd, err = BuildCommand(r)
		if err == nil {
			result.ExitCode, err = e.Manager.Command(ctx, r.HostID, cmd, cap, cap)
		}
	}
	result.Output = cap.prefix.String()
	if cap.err != nil {
		err = cap.err
	}
	result.FinishedAt = time.Now().UTC()
	result.Status = "succeeded"
	if err != nil {
		result.Error = err.Error()
		result.Status = "failed"
		if result.ExitCode < 0 && !localfiles.IsOperation(r.Operation) && r.Operation != "file_hash" && r.Operation != "file_read" && r.Operation != "terminal_read" && r.Operation != "terminal_connect" {
			result.Status = "unknown"
		}
	}
	if saveErr := e.Store.Finish(result); saveErr != nil {
		return result, saveErr
	}
	return result, nil
}
