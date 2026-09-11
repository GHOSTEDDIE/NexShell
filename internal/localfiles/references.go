package localfiles

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveReference accepts user-provided filesystem paths and local file URLs.
func ResolveReference(value string) (string, error) {
	p := strings.TrimSpace(value)
	if strings.HasPrefix(p, "file://") {
		u, err := url.Parse(p)
		if err != nil {
			return "", err
		}
		if u.Host != "" && u.Host != "localhost" {
			return "", os.ErrInvalid
		}
		p = filepath.FromSlash(u.Path)
		if len(p) > 2 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	if err := ValidatePath(p); err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", os.ErrInvalid
	}
	return real, nil
}

var referenceTokens = regexp.MustCompile("`[^`\\n]+`|\"[^\"\\n]+\"|'[^'\\n]+'|(?:file://|~/|/|[A-Za-z]:[\\\\/])[^\\s`\"'<>，。；]+")

// References only inspects paths present in the user's composer, never model output.
func References(text string) []string {
	var candidates []string
	// A line containing only a path may contain unquoted spaces (for example Finder's Copy Pathname).
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if _, err := ResolveReference(line); err == nil {
			candidates = append(candidates, line)
			continue
		}
		for _, match := range referenceTokens.FindAllString(line, -1) {
			candidates = append(candidates, strings.Trim(match, "`\"'"))
		}
	}
	seen := map[string]bool{}
	var paths []string
	for _, candidate := range candidates {
		p, err := ResolveReference(candidate)
		if err != nil || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}
