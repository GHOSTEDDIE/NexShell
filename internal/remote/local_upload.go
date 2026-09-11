package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/localfiles"
	"io"
	"os"
	"path/filepath"
)

func (m *Manager) FileHash(ctx context.Context, host, p string) (string, int64, error) {
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return "", 0, err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if err = canonicalFile(ctx, c, p); err != nil {
		return "", 0, err
	}
	f, err := c.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	if !stat.Mode().IsRegular() {
		return "", 0, fmt.Errorf("目标不是普通文件")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, &contextReader{ctx: ctx, r: f})
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}
func (e *Executor) uploadLocal(ctx context.Context, r domain.Request, out io.Writer) (started bool, err error) {
	staged, err := localfiles.Snapshot(ctx, r.LocalPath, r.SourceHash, filepath.Join(e.Store.Dir, "artifacts"))
	if err != nil {
		return false, err
	}
	defer os.Remove(staged)
	if err = checkAuthority(ctx); err != nil {
		return false, err
	}
	c, err := e.Manager.SFTP(ctx, r.HostID)
	if err != nil {
		return false, err
	}
	if err = canonicalFile(ctx, c, r.Resource); err != nil {
		c.Close()
		return false, err
	}
	mode := r.UploadMode
	if mode == "" {
		mode = "new"
	}
	if mode == "new" {
		_, err = c.Lstat(r.Resource)
		if err == nil {
			c.Close()
			return false, fmt.Errorf("目标文件已存在，请明确选择 replace 或 resume 并重新确认")
		}
		if !os.IsNotExist(err) {
			c.Close()
			return false, err
		}
	}
	c.Close()
	if mode == "new" {
		err = e.Manager.UploadNew(ctx, r.HostID, staged, r.Resource, nil)
	} else {
		err = e.Manager.Transfer(ctx, r.HostID, staged, r.Resource, true, mode == "resume", nil)
	}
	if err != nil {
		return true, err
	}
	_, err = fmt.Fprintf(out, "上传并校验完成：%s\nsha256=%s", r.Resource, r.SourceHash)
	return true, err
}
