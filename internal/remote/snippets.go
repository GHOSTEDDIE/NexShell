package remote

import (
	"fmt"
	"regexp"
	"sort"
)

var parameterPattern = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

func Parameters(template string) []string {
	set := map[string]bool{}
	for _, m := range parameterPattern.FindAllStringSubmatch(template, -1) {
		set[m[1]] = true
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func ExpandSnippet(template string, values map[string]string) (string, error) {
	for _, name := range Parameters(template) {
		if _, ok := values[name]; !ok {
			return "", fmt.Errorf("缺少参数 %s", name)
		}
	}
	return parameterPattern.ReplaceAllStringFunc(template, func(match string) string {
		name := parameterPattern.FindStringSubmatch(match)[1]
		return Quote(values[name])
	}), nil
}
