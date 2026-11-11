package ui

import "testing"

func TestTransferProgressText(t *testing.T) {
	tests := []struct {
		name        string
		transferred int64
		total       int64
		want        string
		value       float64
	}{
		{"screenshot size", 343064576, 686129152, "327.17 MB / 654.34 MB · 50%", 0.5},
		{"gigabytes", 1 << 30, 2 << 30, "1.00 GB / 2.00 GB · 50%", 0.5},
		{"resumed", 200, 1000, "200 B / 1000 B · 20%", 0.2},
		{"empty", 0, 0, "0 B / 0 B · 0%", 0},
		{"overshoot", 1200, 1000, "1000 B / 1000 B · 100%", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transferProgressText(tt.transferred, tt.total); got != tt.want {
				t.Fatalf("text = %q, want %q", got, tt.want)
			}
			if got := transferProgressValue(tt.transferred, tt.total); got != tt.value {
				t.Fatalf("value = %v, want %v", got, tt.value)
			}
		})
	}
}
