package ui

import "fmt"

func transferProgressValue(transferred, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(max(0, min(transferred, total))) / float64(total)
}

func transferProgressText(transferred, total int64) string {
	transferred = max(0, transferred)
	total = max(0, total)
	if total > 0 {
		transferred = min(transferred, total)
	}
	percent := int(transferProgressValue(transferred, total) * 100)
	return fmt.Sprintf("%s / %s · %d%%", transferSize(transferred), transferSize(total), percent)
}

func transferSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
