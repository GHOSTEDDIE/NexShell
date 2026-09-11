//go:build (!darwin && !windows) || ci

package platforminput

func DropPosition(uintptr) (float64, float64, bool) { return 0, 0, false }
