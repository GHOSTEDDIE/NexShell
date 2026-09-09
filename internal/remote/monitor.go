package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Metric struct {
	Value any    `json:"value,omitempty"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}
type NetworkRate struct{ ReceiveBytesPerSecond, SendBytesPerSecond float64 }
type Snapshot struct {
	At                    time.Time `json:"at"`
	CPU                   Metric    `json:"cpu"`
	Memory                Metric    `json:"memory"`
	Load                  Metric    `json:"load"`
	Network               Metric    `json:"network"`
	Disks                 Metric    `json:"disks"`
	Processes             Metric    `json:"processes"`
	Ports                 Metric    `json:"ports"`
	Uptime                Metric    `json:"uptime"`
	System                Metric    `json:"system"`
	TotalTicks, IdleTicks uint64
	Net                   map[string][2]uint64
}

// Each section carries its own exit status; a failed collector is not a zero metric.
const monitorScript = `export LC_ALL=C
section() { printf '\n@@%s\n' "$1"; shift; "$@" 2>&1; printf '\n@@exit:%s\n' "$?"; }
section cpu cat /proc/stat
section memory cat /proc/meminfo
section load cat /proc/loadavg
section network cat /proc/net/dev
section disks df -Pk
section processes ps -eo pid,user,pcpu,pmem,comm --sort=-pcpu
section ports ss -tunap
section system uname -a
section uptime cat /proc/uptime
`

func (m *Manager) Monitor(ctx context.Context, host string, previous *Snapshot) (Snapshot, error) {
	var b bytes.Buffer
	_, err := m.Command(ctx, host, monitorScript, &b, &b)
	if err != nil {
		return Snapshot{}, err
	}
	return ParseSnapshot(b.String(), previous)
}
func ParseSnapshot(text string, prev *Snapshot) (Snapshot, error) {
	s := Snapshot{At: time.Now(), Net: map[string][2]uint64{}}
	parts := map[string]Metric{}
	var active string
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "@@exit:") {
			code := strings.TrimPrefix(line, "@@exit:")
			v := Metric{Value: strings.TrimSpace(strings.Join(lines, "\n")), State: "ok"}
			if code != "0" {
				v.State = "error"
				v.Error = fmt.Sprint(v.Value)
				v.Value = nil
				if code == "127" {
					v.State = "unsupported"
				}
			}
			parts[active] = v
			active = ""
			lines = nil
			continue
		}
		if strings.HasPrefix(line, "@@") {
			active = strings.TrimPrefix(line, "@@")
			lines = nil
			continue
		}
		if active != "" {
			lines = append(lines, line)
		}
	}
	get := func(name string) Metric {
		if m, ok := parts[name]; ok {
			return m
		}
		return Metric{State: "unsupported", Error: "采集结果缺少 " + name}
	}
	s.CPU = get("cpu")
	s.Memory = get("memory")
	s.Load = get("load")
	s.Network = get("network")
	s.Disks = get("disks")
	s.Processes = get("processes")
	s.Ports = get("ports")
	s.System = get("system")
	s.Uptime = get("uptime")
	if s.CPU.State == "ok" {
		fields := strings.Fields(strings.Split(fmt.Sprint(s.CPU.Value), "\n")[0])
		if len(fields) < 5 || fields[0] != "cpu" {
			return s, errors.New("invalid /proc/stat")
		}
		for i := 1; i < len(fields) && i <= 8; i++ {
			n, e := strconv.ParseUint(fields[i], 10, 64)
			if e != nil {
				return s, e
			}
			s.TotalTicks += n
			if i == 4 || i == 5 {
				s.IdleTicks += n
			}
		}
		s.CPU = Metric{State: "warming_up"}
		if prev != nil && s.TotalTicks > prev.TotalTicks && s.IdleTicks >= prev.IdleTicks {
			total := s.TotalTicks - prev.TotalTicks
			idle := s.IdleTicks - prev.IdleTicks
			if idle <= total {
				s.CPU = Metric{State: "ok", Value: 100 * float64(total-idle) / float64(total)}
			}
		}
	}
	if s.Memory.State == "ok" {
		mem := map[string]uint64{}
		for _, l := range strings.Split(fmt.Sprint(s.Memory.Value), "\n") {
			f := strings.Fields(l)
			if len(f) >= 2 {
				n, e := strconv.ParseUint(f[1], 10, 64)
				if e == nil {
					mem[strings.TrimSuffix(f[0], ":")] = n * 1024
				}
			}
		}
		total, ok := mem["MemTotal"]
		avail, ok2 := mem["MemAvailable"]
		if !ok || !ok2 || total == 0 || avail > total {
			s.Memory = Metric{State: "unsupported", Error: "缺少有效内存计数"}
		} else {
			s.Memory.Value = map[string]uint64{"total": total, "available": avail, "used": total - avail}
		}
	}
	if s.Network.State == "ok" {
		for _, l := range strings.Split(fmt.Sprint(s.Network.Value), "\n") {
			name, rest, ok := strings.Cut(l, ":")
			if !ok {
				continue
			}
			f := strings.Fields(rest)
			if len(f) < 9 {
				continue
			}
			rx, e := strconv.ParseUint(f[0], 10, 64)
			tx, e2 := strconv.ParseUint(f[8], 10, 64)
			if e == nil && e2 == nil {
				s.Net[strings.TrimSpace(name)] = [2]uint64{rx, tx}
			}
		}
		s.Network = Metric{State: "warming_up"}
		if prev != nil && s.At.After(prev.At) {
			rates := map[string]NetworkRate{}
			elapsed := s.At.Sub(prev.At).Seconds()
			for name, counts := range s.Net {
				if old, ok := prev.Net[name]; ok && counts[0] >= old[0] && counts[1] >= old[1] {
					rates[name] = NetworkRate{float64(counts[0]-old[0]) / elapsed, float64(counts[1]-old[1]) / elapsed}
				}
			}
			s.Network = Metric{State: "ok", Value: rates}
		}
	}
	return s, nil
}
