package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/pkg/sftp"
	"io"
	"os"
	"path"
	"time"
)

type FileEntry struct {
	Name     string
	Size     int64
	Mode     string
	Modified time.Time
	IsDir    bool
}

func (m *Manager) SFTP(ctx context.Context, host string) (*sftp.Client, error) {
	c, err := m.Connect(ctx, host)
	if err != nil {
		return nil, err
	}
	return sftp.NewClient(c)
}
func (m *Manager) List(ctx context.Context, host, dir string) ([]FileEntry, error) {
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	f, err := c.ReadDirContext(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(f))
	for _, i := range f {
		out = append(out, FileEntry{i.Name(), i.Size(), i.Mode().String(), i.ModTime(), i.IsDir()})
	}
	return out, nil
}
func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (m *Manager) ReadFile(ctx context.Context, host, p string) ([]byte, string, error) {
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return nil, "", err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if err = canonicalFile(ctx, c, p); err != nil {
		return nil, "", err
	}
	f, err := c.Open(p)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4*1024*1024+1))
	if len(b) > 4*1024*1024 {
		return nil, "", errors.New("文件超过 4 MiB，请下载后编辑")
	}
	return b, Hash(b), err
}

// WriteFile uses a compare-before-replace check and a same-directory atomic rename.
// A concurrent third-party write between the check and rename cannot be locked over SFTP.
func (m *Manager) WriteFile(ctx context.Context, host, p, expected string, b []byte) (string, error) {
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return "", err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if err = canonicalFile(ctx, c, p); err != nil {
		return "", err
	}
	if !path.IsAbs(p) || path.Clean(p) == "/" {
		return "", errors.New("文件路径必须是绝对路径")
	}
	current, err := c.Open(p)
	mode := os.FileMode(0600)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var old []byte
	if exists {
		defer current.Close()
		info, e := current.Stat()
		if e != nil {
			return "", e
		}
		mode = info.Mode().Perm()
		old, err = io.ReadAll(io.LimitReader(current, 4*1024*1024+1))
		if err != nil {
			return "", err
		}
		if len(old) > 4*1024*1024 {
			return "", errors.New("文件过大")
		}
		if expected == "" || Hash(old) != expected {
			return "", errors.New("文件已变化，请重新读取后保存")
		}
	} else if expected != "" && expected != "new" {
		return "", errors.New("原文件已被删除")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	stamp := fmt.Sprint(time.Now().UnixNano())
	backup := ""
	if exists {
		backup = p + ".nexshell-backup-" + stamp
		bf, e := c.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
		if e != nil {
			return "", e
		}
		_ = bf.Chmod(0600)
		_, e = bf.Write(old)
		ce := bf.Close()
		if e != nil {
			return "", e
		}
		if ce != nil {
			return "", ce
		}
	}
	tmp := p + ".nexshell-" + stamp
	f, err := c.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
	if err != nil {
		return backup, err
	}
	defer c.Remove(tmp)
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return backup, err
	}
	_, err = f.Write(b)
	ce := f.Close()
	if err != nil {
		return backup, err
	}
	if ce != nil {
		return backup, ce
	}
	if err = ctx.Err(); err != nil {
		return backup, err
	}
	if exists {
		if _, ok := c.HasExtension("posix-rename@openssh.com"); !ok {
			return backup, errors.New("服务器不支持原子覆盖，未修改原文件")
		}
		err = c.PosixRename(tmp, p)
	} else {
		err = c.Rename(tmp, p)
	}
	return backup, err
}

type contextReader struct {
	ctx      context.Context
	r        io.Reader
	progress func(int64)
	n        int64
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, e := r.r.Read(b)
	r.n += int64(n)
	if r.progress != nil {
		r.progress(r.n)
	}
	return n, e
}
func (m *Manager) Transfer(ctx context.Context, host, local, remote string, upload, resume bool, progress func(int64)) error {
	return m.transfer(ctx, host, local, remote, upload, resume, false, progress)
}

// UploadNew never overwrites a file created after the upload was queued.
func (m *Manager) UploadNew(ctx context.Context, host, local, remote string, progress func(int64)) error {
	return m.transfer(ctx, host, local, remote, true, false, true, progress)
}
func (m *Manager) transfer(ctx context.Context, host, local, remote string, upload, resume, exclusive bool, progress func(int64)) error {
	c, err := m.SFTP(ctx, host)
	if err != nil {
		return err
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	var src io.ReadSeekCloser
	var dst interface {
		io.WriteCloser
		io.Seeker
	}
	var total, offset int64
	if upload {
		f, e := os.Open(local)
		if e != nil {
			return e
		}
		src = f
		defer src.Close()
		i, e := f.Stat()
		if e != nil {
			return e
		}
		total = i.Size()
		flags := os.O_CREATE | os.O_WRONLY
		if exclusive {
			flags |= os.O_EXCL
		}
		if !resume {
			flags |= os.O_TRUNC
		}
		rf, e := c.OpenFile(remote, flags)
		if e != nil {
			return e
		}
		dst = rf
		defer dst.Close()
		if resume {
			if i, e := rf.Stat(); e == nil {
				offset = i.Size()
			} else {
				return e
			}
		}
	} else {
		rf, e := c.Open(remote)
		if e != nil {
			return e
		}
		src = rf
		defer src.Close()
		i, e := rf.Stat()
		if e != nil {
			return e
		}
		total = i.Size()
		flags := os.O_CREATE | os.O_WRONLY
		if exclusive {
			flags |= os.O_EXCL
		}
		if !resume {
			flags |= os.O_TRUNC
		}
		f, e := os.OpenFile(local, flags, 0600)
		if e != nil {
			return e
		}
		dst = f
		defer dst.Close()
		if resume {
			if i, e := f.Stat(); e == nil {
				offset = i.Size()
			} else {
				return e
			}
		}
	}
	if offset > total {
		return errors.New("续传目标比源文件大")
	}
	if offset > 0 {
		// Compare the complete existing prefix, not just length or modification time.
		var existing io.ReadCloser
		if upload {
			existing, err = c.Open(remote)
		} else {
			existing, err = os.Open(local)
		}
		if err != nil {
			return err
		}
		a, b := sha256.New(), sha256.New()
		_, err = io.CopyN(a, src, offset)
		if err == nil {
			_, err = io.CopyN(b, existing, offset)
		}
		existing.Close()
		if err != nil {
			return err
		}
		if !equalHash(a.Sum(nil), b.Sum(nil)) {
			return errors.New("续传内容不一致，未覆盖已有文件")
		}
	}
	if _, err = src.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if _, err = dst.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(dst, &contextReader{ctx: ctx, r: src, n: offset, progress: progress})
	if err != nil {
		return err
	}
	if err = dst.Close(); err != nil {
		return err
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	sourceHash := sha256.New()
	if _, err = io.Copy(sourceHash, &contextReader{ctx: ctx, r: src}); err != nil {
		return err
	}
	var check io.ReadCloser
	if upload {
		check, err = c.Open(remote)
	} else {
		check, err = os.Open(local)
	}
	if err != nil {
		return err
	}
	defer check.Close()
	targetHash := sha256.New()
	if _, err = io.Copy(targetHash, &contextReader{ctx: ctx, r: check}); err != nil {
		return err
	}
	if !equalHash(sourceHash.Sum(nil), targetHash.Sum(nil)) {
		return errors.New("传输后的文件校验不一致")
	}
	return nil
}
func equalHash(a, b []byte) bool { return hex.EncodeToString(a) == hex.EncodeToString(b) }
