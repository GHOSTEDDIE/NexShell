package remote

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DiskUsage preserves df's reported available space and percentage; reserved
// blocks mean they cannot be derived reliably from total minus used.
type DiskUsage struct {
	Filesystem, Mount string
	TotalKiB, UsedKiB uint64
	AvailableKiB      int64
	Percent           uint64
}

var diskUsageLine = regexp.MustCompile(`^(.+?)\s+(\d+)\s+(\d+)\s+(-?\d+)\s+(\d+)%\s+(.+)$`)

func ParseDiskUsage(output string) ([]DiskUsage, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "Filesystem") {
		return nil, fmt.Errorf("磁盘数据缺少表头")
	}
	disks := make([]DiskUsage, 0, len(lines)-1)
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := diskUsageLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			return nil, fmt.Errorf("磁盘数据第 %d 行格式错误", i+2)
		}
		total, e1 := strconv.ParseUint(m[2], 10, 64)
		used, e2 := strconv.ParseUint(m[3], 10, 64)
		available, e3 := strconv.ParseInt(m[4], 10, 64)
		percent, e4 := strconv.ParseUint(m[5], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			return nil, fmt.Errorf("磁盘数据第 %d 行数值无效", i+2)
		}
		disks = append(disks, DiskUsage{Filesystem: m[1], Mount: m[6], TotalKiB: total, UsedKiB: used, AvailableKiB: available, Percent: percent})
	}
	return disks, nil
}
