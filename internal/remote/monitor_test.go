package remote

import (
	"strings"
	"testing"
)

func TestHealthyMetricsRemainCorrect(t *testing.T) {
	sample := "\n@@cpu\ncpu  10 0 10 80 0 0 0 0 2 0\n@@exit:0\n@@memory\nMemTotal: 1000 kB\nMemAvailable: 400 kB\n@@exit:0\n@@network\n eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n@@exit:0\n@@ports\npermission denied\n@@exit:1\n"
	first, e := ParseSnapshot(sample, nil)
	if e != nil {
		t.Fatal(e)
	}
	if first.CPU.State != "warming_up" || first.TotalTicks != 100 {
		t.Fatal(first)
	}
	next := strings.Replace(sample, "10 0 10 80", "20 0 20 160", 1)
	second, e := ParseSnapshot(next, &first)
	if e != nil {
		t.Fatal(e)
	}
	if second.CPU.Value.(float64) != 20 {
		t.Fatal(second.CPU)
	}
	memory := second.Memory.Value.(map[string]uint64)
	if memory["used"] != 600*1024 {
		t.Fatal(memory)
	}
	if second.Ports.State != "error" || second.Ports.Value != nil {
		t.Fatal("failure represented as a zero/valid metric")
	}
	if second.Net["eth0"] != [2]uint64{100, 200} {
		t.Fatal(second.Net)
	}
}
func TestShellQuotingAndNames(t *testing.T) {
	if Quote("a'b") != "'a'\"'\"'b'" {
		t.Fatal(Quote("a'b"))
	}
}
