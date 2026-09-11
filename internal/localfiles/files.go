// Package localfiles provides read-only, rooted access to user-selected local paths.
package localfiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type rootsKey struct{}

func WithRoots(ctx context.Context, roots []string) context.Context {
	return context.WithValue(ctx, rootsKey{}, append([]string(nil), roots...))
}
func Roots(ctx context.Context) []string { v, _ := ctx.Value(rootsKey{}).([]string); return v }
func IsOperation(op string) bool {
	return op == "local_stat" || op == "local_list" || op == "local_read"
}
func ValidatePath(p string) error {
	if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return errors.New("请使用规范的本机绝对路径")
	}
	return nil
}

// CanonicalPath resolves directory aliases before an execution request is approved.
// Keep the final component unchanged: file symlinks still require the real path.
func CanonicalPath(p string) (string, error) {
	if err := ValidatePath(p); err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	if filepath.Dir(p) == p {
		return parent, nil
	}
	return filepath.Join(parent, filepath.Base(p)), nil
}
func Within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func CanonicalRoots(roots []string) ([]string, error) {
	out := []string{}
	for _, p := range roots {
		if err := ValidatePath(p); err != nil {
			return nil, err
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(real)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("本地授权路径不是目录：%s", p)
		}
		out = append(out, real)
	}
	return out, nil
}
func open(p string, roots []string) (*os.File, error) {
	if err := ValidatePath(p); err != nil {
		return nil, err
	}
	root := filepath.Dir(p)
	name := filepath.Base(p)
	if root == p {
		name = "."
	}
	// An automatically authorized directory is opened as a root; links cannot escape it.
	for _, candidate := range roots {
		if Within(candidate, p) {
			root = candidate
			name, _ = filepath.Rel(root, p)
			break
		}
	}
	bounded, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer bounded.Close()
	info, err := bounded.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil, errors.New("请使用普通文件或目录的实际路径")
	}
	return bounded.Open(name)
}

type Entry struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Directory bool   `json:"directory"`
}
type Info struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Modified   time.Time `json:"modified"`
	Directory  bool      `json:"directory"`
	SHA256     string    `json:"sha256,omitempty"`
	Content    string    `json:"content,omitempty"`
	Entries    []Entry   `json:"entries,omitempty"`
	NextOffset int64     `json:"next_offset,omitempty"`
	Truncated  bool      `json:"truncated"`
}
type checkedReader struct {
	ctx context.Context
	r   io.Reader
}

func (r checkedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func Inspect(ctx context.Context, p, operation string, offsets ...int64) (Info, error) {
	if !IsOperation(operation) {
		return Info{}, errors.New("不支持的本机文件操作")
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	f, err := open(p, Roots(ctx))
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return Info{}, err
	}
	info := Info{Path: p, Size: stat.Size(), Modified: stat.ModTime(), Directory: stat.IsDir()}
	if operation == "local_list" {
		if !stat.IsDir() {
			return info, errors.New("指定路径不是目录")
		}
		entries, err := f.ReadDir(101)
		if err != nil && err != io.EOF {
			return info, err
		}
		if len(entries) > 100 {
			info.Truncated = true
			entries = entries[:100]
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return info, err
			}
			meta, err := entry.Info()
			if err != nil {
				return info, err
			}
			info.Entries = append(info.Entries, Entry{entry.Name(), meta.Size(), entry.IsDir()})
			encoded, _ := json.Marshal(info)
			if len(encoded) > 24000 {
				info.Entries = info.Entries[:len(info.Entries)-1]
				info.Truncated = true
				break
			}
		}
		return info, nil
	}
	if stat.IsDir() {
		if operation == "local_stat" {
			return info, nil
		}
		return info, errors.New("请使用目录查询")
	}
	if !stat.Mode().IsRegular() {
		return info, errors.New("只支持普通文件")
	}
	if operation == "local_read" {
		var offset int64
		if len(offsets) > 0 {
			offset = offsets[0]
		}
		if offset < 0 || offset > stat.Size() {
			return info, errors.New("读取偏移量超出文件范围")
		}
		if _, err = f.Seek(offset, io.SeekStart); err != nil {
			return info, err
		}
		b, err := io.ReadAll(io.LimitReader(checkedReader{ctx, f}, 4097))
		if err != nil {
			return info, err
		}
		if len(b) > 4096 {
			info.Truncated = true
			b = b[:4096]
			for len(b) > 0 && !utf8.Valid(b) && len(b) > 4092 {
				b = b[:len(b)-1]
			}
		}
		if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
			return info, errors.New("不是 UTF-8 文本；部署包请使用 local_stat 获取校验值后上传")
		}
		info.Content = string(b)
		info.NextOffset = offset + int64(len(b))
		return info, nil
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, checkedReader{ctx, f}); err != nil {
		return info, err
	}
	info.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return info, nil
}

// Snapshot streams the approved bytes into a private file before touching the remote target.
func Snapshot(ctx context.Context, p, expected, directory string) (string, error) {
	f, err := open(p, Roots(ctx))
	if err != nil {
		return "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", errors.New("部署包必须是普通文件")
	}
	staged, err := os.CreateTemp(directory, ".upload-*")
	if err != nil {
		return "", err
	}
	name := staged.Name()
	keep := false
	defer func() {
		staged.Close()
		if !keep {
			os.Remove(name)
		}
	}()
	hash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(staged, hash), checkedReader{ctx, f}); err != nil {
		return "", err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return "", errors.New("本地文件已变化，请重新获取校验值并确认上传")
	}
	if err = staged.Close(); err != nil {
		return "", err
	}
	keep = true
	return name, nil
}
