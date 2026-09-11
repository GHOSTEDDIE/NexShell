//go:build (!darwin && !windows) || ci

package platforminput

// Other desktops expose URI text through Fyne; file picking and dropping are also supported.
func ClipboardFiles() ([]string, error) { return nil, nil }
