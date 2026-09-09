package ui

// shortID abbreviates identifiers for labels only; stored IDs remain intact.
func shortID(id string) string {
	characters := []rune(id)
	return string(characters[:min(8, len(characters))])
}
