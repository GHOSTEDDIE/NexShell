package remote

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ProcessInfo and PortInfo retain the complete source values; compact UI
// columns are a presentation choice, not a lossy parsing format.
type ProcessInfo struct {
	PID         int
	User, Name  string
	CPU, Memory float64
}
type PortOwner struct {
	Name    string
	PID, FD int
}

var portOwnerPattern = regexp.MustCompile(`\("((?:[^"\\]|\\.)*)",pid=([0-9]+),fd=([0-9]+)\)`)

type PortInfo struct {
	Owners                                []PortOwner
	Protocol, State, Local, Peer, Process string
	ReceiveQueue, SendQueue               uint64
}

// takeFields splits only the leading fields. Command names and process details
// in the remaining column may legitimately contain whitespace.
func takeFields(line string, n int) ([]string, string, bool) {
	out := make([]string, 0, n)
	rest := strings.TrimSpace(line)
	for i := 0; i < n; i++ {
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			if i == n-1 && rest != "" {
				out = append(out, rest)
				return out, "", true
			}
			return nil, "", false
		}
		out = append(out, rest[:end])
		rest = strings.TrimLeft(rest[end:], " \t")
	}
	return out, rest, true
}
func ParseProcesses(output string) ([]ProcessInfo, error) {
	rows := []ProcessInfo{}
	for i, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "PID ") || strings.HasPrefix(line, "PID\t") {
			continue
		}
		f, name, ok := takeFields(line, 4)
		if !ok || name == "" {
			return nil, fmt.Errorf("进程第 %d 行格式错误", i+1)
		}
		pid, e1 := strconv.Atoi(f[0])
		cpu, e2 := strconv.ParseFloat(f[2], 64)
		mem, e3 := strconv.ParseFloat(f[3], 64)
		if e1 != nil || e2 != nil || e3 != nil || pid <= 0 || cpu < 0 || mem < 0 || math.IsNaN(cpu) || math.IsNaN(mem) || math.IsInf(cpu, 0) || math.IsInf(mem, 0) {
			return nil, fmt.Errorf("进程第 %d 行数值无效", i+1)
		}
		rows = append(rows, ProcessInfo{PID: pid, User: f[1], Name: name, CPU: cpu, Memory: mem})
	}
	return rows, nil
}
func ParsePorts(output string) ([]PortInfo, error) {
	rows := []PortInfo{}
	for i, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Netid ") || strings.HasPrefix(line, "Netid\t") {
			continue
		}
		f, process, ok := takeFields(line, 6)
		if !ok || (f[0] != "tcp" && f[0] != "udp" && f[0] != "tcp6" && f[0] != "udp6") {
			return nil, fmt.Errorf("端口第 %d 行格式错误", i+1)
		}
		rx, e1 := strconv.ParseUint(f[2], 10, 64)
		tx, e2 := strconv.ParseUint(f[3], 10, 64)
		if e1 != nil || e2 != nil || !strings.Contains(f[4], ":") || !strings.Contains(f[5], ":") {
			return nil, fmt.Errorf("端口第 %d 行字段无效", i+1)
		}
		owners := []PortOwner{}
		for _, match := range portOwnerPattern.FindAllStringSubmatch(process, -1) {
			name, err := strconv.Unquote(`"` + match[1] + `"`)
			if err != nil {
				return nil, fmt.Errorf("端口第 %d 行进程名称无效", i+1)
			}
			pid, e1 := strconv.Atoi(match[2])
			fd, e2 := strconv.Atoi(match[3])
			if e1 != nil || e2 != nil || pid <= 0 {
				return nil, fmt.Errorf("端口第 %d 行进程 ID 无效", i+1)
			}
			owners = append(owners, PortOwner{Name: name, PID: pid, FD: fd})
		}
		rows = append(rows, PortInfo{Owners: owners, Protocol: f[0], State: f[1], Local: f[4], Peer: f[5], ReceiveQueue: rx, SendQueue: tx, Process: process})
	}
	return rows, nil
}
