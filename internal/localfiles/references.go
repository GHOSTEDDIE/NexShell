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
		fileURL := p
		// url.Parse treats the colon in file://D:%5Cpath as a port
		// separator and rejects it. Normalize this Windows form to the
		// standards-compatible file:///D:%5Cpath form first.
		authority := strings.TrimPrefix(p, "file://")
		if len(authority) >= 2 && authority[1] == ':' && isDriveLetter(authority[0]) {
			fileURL = "file:///" + authority
		}
		u, err := url.Parse(fileURL)
		if err != nil {
			return "", err
		}
		if u.Host != "" && u.Host != "localhost" {
			// Windows file URLs produced from a drive-letter path may encode
			// the drive as the authority: file://D:%5Cpath%5Cfile. It is
			// local path syntax, not a network host.
			if len(u.Host) != 2 || u.Host[1] != ':' ||
				(u.Host[0] < 'A' || u.Host[0] > 'Z') && (u.Host[0] < 'a' || u.Host[0] > 'z') {
				return "", os.ErrInvalid
			}
			p = u.Host + u.Path
		} else {
			p = u.Path
		}
		p = filepath.FromSlash(p)
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

func isDriveLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
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
