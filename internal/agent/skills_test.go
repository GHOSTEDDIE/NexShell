package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillIsolation(t *testing.T) {
	root := t.TempDir()
	s := SkillLibrary{Root: root}
	list, e := s.Read("")
	if e != nil || !strings.Contains(list, "builtin:linux-diagnostics") {
		t.Fatal(list, e)
	}
	if _, e = s.Read("../outside"); e == nil {
		t.Fatal("traversal accepted")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("outside"), 0600)
	if e = os.Symlink(outside, filepath.Join(root, "escape")); e == nil {
		if _, e = s.Read("local:escape"); e == nil {
			t.Fatal("symlink escaped library")
		}
	}
}
