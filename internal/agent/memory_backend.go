package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/bmatcuk/doublestar/v4"
	fs "github.com/cloudwego/eino/adk/filesystem"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MemoryRecord struct {
	Path, Content string
	UpdatedAt     time.Time
}
type MemorySettings struct {
	Disabled   bool
	Generation uint64
}
type memoryBackend struct {
	revisions                    sync.Map
	s                            *Service
	scope, root                  string
	generation, globalGeneration uint64
}

func (s *Service) memorySettings(scope string) (MemorySettings, error) {
	var v MemorySettings
	err := s.Store.Load("memory_settings", scope, &v)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return v, err
}
func (s *Service) MemoryEnabled() (bool, error) {
	s.memoryMu.RLock()
	defer s.memoryMu.RUnlock()
	v, e := s.memorySettings("global")
	return !v.Disabled, e
}
func (s *Service) SetMemoryEnabled(enabled bool) error {
	s.memoryMu.Lock()
	defer s.memoryMu.Unlock()
	v, e := s.memorySettings("global")
	if e != nil {
		return e
	}
	v.Disabled = !enabled
	v.Generation++
	return s.Store.Put("memory_settings", "global", v)
}
func (s *Service) ListMemory(scope string) ([]MemoryRecord, error) {
	s.memoryMu.RLock()
	defer s.memoryMu.RUnlock()
	return store.All[MemoryRecord](s.Store, "memory:"+scope)
}
func (s *Service) DeleteMemory(scope, name string) error {
	s.memoryMu.Lock()
	defer s.memoryMu.Unlock()
	v, e := s.memorySettings(scope)
	if e != nil {
		return e
	}
	v.Generation++
	if e = s.Store.Put("memory_settings", scope, v); e != nil {
		return e
	}
	return s.Store.Delete("memory:"+scope, name)
}
func (s *Service) newMemoryBackend(scope string) (*memoryBackend, error) {
	s.memoryMu.RLock()
	defer s.memoryMu.RUnlock()
	v, e := s.memorySettings(scope)
	if e != nil {
		return nil, e
	}
	g, e := s.memorySettings("global")
	if e != nil {
		return nil, e
	}
	return &memoryBackend{s: s, scope: scope, root: filepath.Join(s.Store.Dir, "memory", domain.Digest(scope)), generation: v.Generation, globalGeneration: g.Generation}, nil
}
func (b *memoryBackend) valid(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	g, e := b.s.memorySettings("global")
	if e != nil {
		return e
	}
	v, e := b.s.memorySettings(b.scope)
	if e != nil {
		return e
	}
	if g.Disabled || g.Generation != b.globalGeneration || v.Generation != b.generation {
		return errors.New("记忆设置已变化，本次读写已取消")
	}
	return nil
}
func (b *memoryBackend) key(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", errors.New("记忆路径无效")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(b.root, p)
	}
	rel, e := filepath.Rel(b.root, filepath.Clean(p))
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.Ext(rel) != ".md" {
		return "", errors.New("记忆路径越界或不是 Markdown")
	}
	return filepath.ToSlash(rel), nil
}
func (b *memoryBackend) Read(ctx context.Context, r *fs.ReadRequest) (*fs.FileContent, error) {
	b.s.memoryMu.RLock()
	defer b.s.memoryMu.RUnlock()
	if e := b.valid(ctx); e != nil {
		return nil, e
	}
	key, e := b.key(r.FilePath)
	if e != nil {
		return nil, e
	}
	var doc MemoryRecord
	if e = b.s.Store.Load("memory:"+b.scope, key, &doc); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			e = os.ErrNotExist
		}
		return nil, e
	}
	b.revisions.Store(key, domain.Digest(doc))
	lines := strings.Split(doc.Content, "\n")
	start := max(0, r.Offset-1)
	if start >= len(lines) {
		return &fs.FileContent{}, nil
	}
	end := len(lines)
	if r.Limit > 0 {
		end = min(end, start+r.Limit)
	}
	return &fs.FileContent{Content: strings.Join(lines[start:end], "\n")}, nil
}
func (b *memoryBackend) GlobInfo(ctx context.Context, r *fs.GlobInfoRequest) ([]fs.FileInfo, error) {
	b.s.memoryMu.RLock()
	defer b.s.memoryMu.RUnlock()
	if e := b.valid(ctx); e != nil {
		return nil, e
	}
	base := r.Path
	if base == "" {
		base = b.root
	}
	if !filepath.IsAbs(base) {
		base = filepath.Join(b.root, base)
	}
	rel, e := filepath.Rel(b.root, filepath.Clean(base))
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("记忆目录越界")
	}
	docs, e := store.All[MemoryRecord](b.s.Store, "memory:"+b.scope)
	if e != nil {
		return nil, e
	}
	out := []fs.FileInfo{}
	for _, doc := range docs {
		name, e := filepath.Rel(base, filepath.Join(b.root, filepath.FromSlash(doc.Path)))
		if e != nil || strings.HasPrefix(name, "..") {
			continue
		}
		match, e := doublestar.Match(r.Pattern, filepath.ToSlash(name))
		if e != nil {
			return nil, e
		}
		if match {
			out = append(out, fs.FileInfo{Path: filepath.Join(b.root, filepath.FromSlash(doc.Path)), Size: int64(len(doc.Content)), ModifiedAt: doc.UpdatedAt.Format(time.RFC3339)})
		}
	}
	return out, nil
}
func (b *memoryBackend) write(ctx context.Context, key, content string) error {
	if e := b.valid(ctx); e != nil {
		return e
	}
	if len(content) > 65536 {
		return errors.New("单条记忆超过 64 KiB")
	}
	if containsCredential(content) {
		return errors.New("记忆不能包含凭据内容")
	}
	doc := MemoryRecord{key, content, time.Now().UTC()}
	if err := b.s.Store.Put("memory:"+b.scope, key, doc); err != nil {
		return err
	}
	b.revisions.Store(key, domain.Digest(doc))
	return nil
}
func (b *memoryBackend) Write(ctx context.Context, r *fs.WriteRequest) error {
	b.s.memoryMu.Lock()
	defer b.s.memoryMu.Unlock()
	key, e := b.key(r.FilePath)
	if e != nil {
		return e
	}
	var existing MemoryRecord
	err := b.s.Store.Load("memory:"+b.scope, key, &existing)
	if err == nil {
		revision, ok := b.revisions.Load(key)
		if !ok || revision != domain.Digest(existing) {
			return errors.New("记忆已存在或已变化，请重新读取后修改")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return b.write(ctx, key, r.Content)
}
func (b *memoryBackend) Edit(ctx context.Context, r *fs.EditRequest) error {
	b.s.memoryMu.Lock()
	defer b.s.memoryMu.Unlock()
	if e := b.valid(ctx); e != nil {
		return e
	}
	key, e := b.key(r.FilePath)
	if e != nil {
		return e
	}
	var doc MemoryRecord
	if e = b.s.Store.Load("memory:"+b.scope, key, &doc); e != nil {
		return e
	}
	count := strings.Count(doc.Content, r.OldString)
	if r.OldString == "" || count == 0 || (!r.ReplaceAll && count != 1) {
		return fmt.Errorf("记忆修改匹配数量无效: %d", count)
	}
	n := 1
	if r.ReplaceAll {
		n = -1
	}
	return b.write(ctx, key, strings.Replace(doc.Content, r.OldString, r.NewString, n))
}
