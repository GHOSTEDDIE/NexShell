//go:build !darwin || ci

package platforminput

const Available = false

func SetAnchor(window uintptr, x, y, width, height, canvasW, canvasH float32, reset bool) {}

func ReducedMotion() bool { return false }
