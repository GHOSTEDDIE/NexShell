package agent

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed runbooks/*/SKILL.md
var runbooks embed.FS

type SkillLibrary struct{ Root string }

func (s SkillLibrary) Read(name string) (string, error) {
	if name == "" {
		entries, e := runbooks.ReadDir("runbooks")
		if e != nil {
			return "", e
		}
		var names []string
		for _, entry := range entries {
			names = append(names, "builtin:"+entry.Name())
		}
		local, e := os.ReadDir(s.Root)
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		for _, entry := range local {
			if entry.IsDir() {
				names = append(names, "local:"+entry.Name())
			}
		}
		sort.Strings(names)
		return strings.Join(names, "\n"), nil
	}
	origin, short, ok := strings.Cut(name, ":")
	if !ok {
		origin = "builtin"
		short = name
	}
	if filepath.Base(short) != short || short == "." || short == ".." || short == "" {
		return "", fmt.Errorf("无效手册名称")
	}
	if origin == "builtin" {
		b, e := runbooks.ReadFile("runbooks/" + short + "/SKILL.md")
		return string(b), e
	}
	if origin != "local" {
		return "", fmt.Errorf("未知手册来源")
	}
	root, e := filepath.EvalSymlinks(s.Root)
	if e != nil {
		return "", e
	}
	p, e := filepath.EvalSymlinks(filepath.Join(root, short, "SKILL.md"))
	if e != nil {
		return "", e
	}
	relative, e := filepath.Rel(root, p)
	if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("手册路径越界")
	}
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 32769))
	if len(b) > 32768 {
		return "", fmt.Errorf("手册超过 32 KiB")
	}
	return string(b), e
}
