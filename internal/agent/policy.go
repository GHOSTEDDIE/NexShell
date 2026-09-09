package agent

import (
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"path"
	"slices"
	"strings"
	"time"
)

type Approval struct {
	InterruptID string         `json:"interrupt_id"`
	Request     domain.Request `json:"request"`
	Digest      string         `json:"digest"`
	GrantID     string         `json:"grant_id"`
	Reason      string         `json:"reason"`
	Approved    bool           `json:"approved"`
	Decided     bool           `json:"decided"`
}

func Evaluate(t domain.Task, h domain.Host, r domain.Request, now time.Time) (bool, error) {
	if t.ID != r.TaskID || !slices.Contains(t.Grant.HostIDs, r.HostID) {
		return false, errors.New("目标主机不在本任务范围内")
	}
	if !now.Before(t.Grant.ExpiresAt) {
		return false, errors.New("任务授权已过期")
	}
	if t.Grant.Identities[h.ID] != domain.Digest(h) {
		return false, errors.New("主机连接或身份配置已变化，请重新授权")
	}
	switch r.Operation {
	case "terminal_connect", "terminal_read", "terminal_write", "observe", "service_status", "logs", "file_read", "file_write", "service_restart", "package_install", "shell":
	default:
		return false, errors.New("未知工具操作")
	}
	if strings.ContainsRune(r.Resource, 0) || strings.ContainsRune(r.Command, 0) {
		return false, errors.New("参数包含空字节")
	}
	if r.Operation == "file_read" || r.Operation == "file_write" {
		if !path.IsAbs(r.Resource) || path.Clean(r.Resource) != r.Resource || r.Resource == "/" {
			return false, errors.New("请使用规范的绝对文件路径")
		}
	}
	// Arbitrary shell cannot inherit a resource-scoped grant. It always binds to an exact request.
	if r.Operation == "shell" || r.Operation == "terminal_write" {
		return false, nil
	}
	if !slices.Contains(t.Grant.Operations, r.Operation) {
		return false, nil
	}
	switch r.Operation {
	case "file_read", "file_write", "service_restart", "package_install":
		return slices.Contains(t.Grant.Resources, r.Resource), nil
	}
	return true, nil
}
func ApprovedFor(a Approval, t domain.Task, r domain.Request) bool {
	return a.Decided && a.Approved && a.Digest == domain.Digest(r) && a.GrantID == t.Grant.ID && a.Request.TaskID == t.ID
}
