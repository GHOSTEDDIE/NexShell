package localfiles

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReadListStatAndBoundary(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "部署.txt")
	data := strings.Repeat("中", 1400)
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := WithRoots(context.Background(), []string{root})
	info, err := Inspect(ctx, p, "local_read")
	if err != nil || !info.Truncated || !utf8.ValidString(info.Content) || len(info.Content) > 4096 || !strings.HasPrefix(data, info.Content) {
		t.Fatalf("preview: %+v %v", info, err)
	}
	tail, err := Inspect(ctx, p, "local_read", info.NextOffset)
	if err != nil || info.Content+tail.Content != data || tail.Truncated {
		t.Fatal("text pagination lost characters", err)
	}
	info, err = Inspect(ctx, p, "local_stat")
	if err != nil || info.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(data))) || info.Content != "" {
		t.Fatalf("metadata: %+v %v", info, err)
	}
	info, err = Inspect(ctx, root, "local_list")
	if err != nil || len(info.Entries) != 1 || info.Entries[0].Name != "部署.txt" {
		t.Fatalf("list: %+v %v", info, err)
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "private"), []byte("secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skip(err)
	}
	if _, err = Inspect(ctx, filepath.Join(root, "escape", "private"), "local_read"); err == nil {
		t.Fatal("symlink escaped grant")
	}
	if _, err = Inspect(ctx, filepath.Join(root, "escape"), "local_list"); err == nil {
		t.Fatal("symlink accepted")
	}
	if Within(root, root+"-other") || ValidatePath(filepath.Join(root, ".")+string(filepath.Separator)+"../private") == nil {
		t.Fatal("path boundary accepted")
	}
	if _, err = Inspect(ctx, p, "local_write"); err == nil {
		t.Fatal("unsupported operation accepted")
	}
}
func TestBinarySnapshotAndChangedSource(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	p := filepath.Join(root, "package.tar.gz")
	data := []byte{0, 1, 255, 4, 5}
	os.WriteFile(p, data, 0600)
	ctx := WithRoots(context.Background(), []string{root})
	if _, err := Inspect(ctx, p, "local_read"); err == nil {
		t.Fatal("binary exposed as text")
	}
	info, err := Inspect(ctx, p, "local_stat")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Snapshot(ctx, p, info.SHA256, stage)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(snapshot)
	if err != nil || string(b) != string(data) {
		t.Fatal("snapshot bytes changed", err)
	}
	os.Remove(snapshot)
	os.WriteFile(p, []byte("changed"), 0600)
	if _, err = Snapshot(ctx, p, info.SHA256, stage); err == nil {
		t.Fatal("changed source accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Snapshot(canceled, p, info.SHA256, stage); err == nil {
		t.Fatal("cancellation ignored")
	}
	entries, _ := os.ReadDir(stage)
	if len(entries) != 0 {
		t.Fatal("staging files leaked")
	}
}

func TestCanonicalDirectoryAlias(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	os.Mkdir(actual, 0700)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Skip(err)
	}
	p, err := CanonicalPath(filepath.Join(alias, "package.bin"))
	if err != nil {
		t.Fatal(err)
	}
	roots, err := CanonicalRoots([]string{alias})
	if err != nil || len(roots) != 1 || !Within(roots[0], p) {
		t.Fatal("alias and grant disagree", p, roots, err)
	}
}

func TestUserFileReferences(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "release 包.tar.gz")
	prefix := filepath.Join(root, "release")
	os.WriteFile(p, []byte{0, 1}, 0600)
	os.WriteFile(prefix, []byte("other"), 0600)
	real, _ := filepath.EvalSymlinks(p)
	for _, text := range []string{p, "部署 `" + p + "` 到服务器", "使用 \"" + p + "\"", (&url.URL{Scheme: "file", Path: p}).String()} {
		got := References(text)
		if len(got) != 1 || got[0] != real {
			t.Fatalf("%q: %v", text, got)
		}
	}
	if got := References("https://example.invalid/package.zip"); len(got) != 0 {
		t.Fatal(got)
	}
	if _, err := ResolveReference("file://remote-server/path"); err == nil {
		t.Fatal("network file URI accepted")
	}
	if _, err := ResolveReference(root); err == nil {
		t.Fatal("directory became file attachment")
	}
}
