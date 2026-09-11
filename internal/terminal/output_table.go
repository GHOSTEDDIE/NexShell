package terminal

import (
	"github.com/charmbracelet/x/ansi"
	"regexp"
	"strings"
	"unicode"
)

type OutputTable struct {
	Headers []string
	Rows    [][]string
}

var tableGap = regexp.MustCompile(` {2,}`)
var shellPrompt = regexp.MustCompile(`^[^\s]*[#$>]\s`)

// ParseOutputTable reads the most recent aligned, uppercase-header command
// table. Column boundaries come from the header, never from spaces inside data.
func ParseOutputTable(lines []string) (OutputTable, bool) {
	for i := len(lines) - 1; i >= 0; i-- {
		header := lines[i]
		gaps := tableGap.FindAllStringIndex(header, -1)
		if len(gaps) == 0 {
			continue
		}
		starts := []int{0}
		for _, gap := range gaps {
			if gap[0] > 0 && gap[1] < len(header) {
				starts = append(starts, gap[1])
			}
		}
		if len(starts) < 2 {
			continue
		}
		headers := make([]string, len(starts))
		valid := true
		columns := make([]int, len(starts))
		for col, start := range starts {
			end := len(header)
			if col+1 < len(starts) {
				end = starts[col+1]
			}
			name := strings.TrimSpace(header[start:end])
			hasLetter := false
			for _, r := range name {
				hasLetter = hasLetter || unicode.IsLetter(r)
			}
			if name == "" || !hasLetter || name != strings.ToUpper(name) {
				valid = false
				break
			}
			headers[col] = name
			columns[col] = ansi.StringWidth(header[:start])
		}
		if !valid {
			continue
		}
		result := OutputTable{Headers: headers}
		for _, line := range lines[i+1:] {
			if strings.TrimSpace(line) == "" || shellPrompt.MatchString(line) {
				break
			}
			cells := splitDisplayColumns(line, columns)
			populated := 0
			for _, cell := range cells {
				if cell != "" {
					populated++
				}
			}
			if populated < 2 {
				break
			}
			result.Rows = append(result.Rows, cells)
		}
		return result, true
	}
	return OutputTable{}, false
}
func splitDisplayColumns(line string, columns []int) []string {
	out := make([]string, len(columns))
	col, position := 0, 0
	var value strings.Builder
	for len(line) > 0 {
		cluster, width := ansi.FirstGraphemeCluster(line, ansi.GraphemeWidth)
		for col+1 < len(columns) && position >= columns[col+1] {
			out[col] = strings.TrimSpace(value.String())
			value.Reset()
			col++
		}
		value.WriteString(cluster)
		position += width
		line = line[len(cluster):]
	}
	out[col] = strings.TrimSpace(value.String())
	return out
}
func (t OutputTable) TSV() string {
	var b strings.Builder
	b.WriteString(strings.Join(t.Headers, "\t"))
	for _, row := range t.Rows {
		b.WriteByte('\n')
		b.WriteString(strings.Join(row, "\t"))
	}
	return b.String()
}
func (c *Core) LatestTable() (OutputTable, bool) {
	c.mu.Lock()
	if c.vt.IsAltScreen() {
		c.mu.Unlock()
		return OutputTable{}, false
	}
	lines := c.vt.LogicalLines()
	c.mu.Unlock()
	return ParseOutputTable(lines)
}
