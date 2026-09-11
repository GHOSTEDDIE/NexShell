package agent

import (
	"encoding/hex"
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
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
	case "upload_local", "file_hash", "terminal_connect", "terminal_read", "terminal_write", "observe", "service_status", "logs", "file_read", "file_write", "service_restart", "package_install", "shell":
	default:
		return false, errors.New("未知工具操作")
	}
	if strings.ContainsRune(r.Resource, 0) || strings.ContainsRune(r.Command, 0) {
		return false, errors.New("参数包含空字节")
	}
	if r.Operation == "file_read" || r.Operation == "file_write" || r.Operation == "upload_local" || r.Operation == "file_hash" {
		if !path.IsAbs(r.Resource) || path.Clean(r.Resource) != r.Resource || r.Resource == "/" {
			return false, errors.New("请使用规范的绝对文件路径")
		}
	}
	if r.Operation == "upload_local" {
		if err := localfiles.ValidatePath(r.LocalPath); err != nil {
			return false, err
		}
		hash, err := hex.DecodeString(r.SourceHash)
		if err != nil || len(hash) != 32 {
			return false, errors.New("请先用 local_files stat 获取完整 SHA-256 校验值")
		}
		if r.UploadMode != "" && r.UploadMode != "new" && r.UploadMode != "replace" && r.UploadMode != "resume" {
			return false, errors.New("上传模式必须为 new、replace 或 resume")
		}
		return false, nil
	}
	// Arbitrary shell cannot inherit a resource-scoped grant. It always binds to an exact request.
	if r.Operation == "shell" || r.Operation == "terminal_write" {
		return false, nil
	}
	if !slices.Contains(t.Grant.Operations, r.Operation) {
		return false, nil
	}
	switch r.Operation {
	case "file_hash", "file_read", "file_write", "service_restart", "package_install":
		return slices.Contains(t.Grant.Resources, r.Resource), nil
	}
	return true, nil
}
func ApprovedFor(a Approval, t domain.Task, r domain.Request) bool {
	return a.Decided && a.Approved && a.Digest == domain.Digest(r) && a.GrantID == t.Grant.ID && a.Request.TaskID == t.ID
}

func EvaluateLocal(t domain.Task, r domain.Request, now time.Time) (bool, error) {
	if r.TaskID != t.ID || r.HostID != "" || !localfiles.IsOperation(r.Operation) {
		return false, errors.New("无效的本地文件请求")
	}
	if !now.Before(t.Grant.ExpiresAt) {
		return false, errors.New("任务授权已过期")
	}
	if err := localfiles.ValidatePath(r.LocalPath); err != nil {
		return false, err
	}
	if r.ReadOffset < 0 || (r.Operation != "local_read" && r.ReadOffset != 0) {
		return false, errors.New("读取偏移量无效")
	}
	for _, root := range t.Grant.LocalRoots {
		if localfiles.Within(root, r.LocalPath) {
			return true, nil
		}
	}
	return false, nil
}
