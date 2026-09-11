//go:build darwin && !ci

package platforminput

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
int nexReducedMotion(void);
void nexInputRect(uintptr_t window, double x, double y, double width, double height, double canvasW, double canvasH, int reset);
*/
import "C"

const Available = true

func SetAnchor(window uintptr, x, y, width, height, canvasW, canvasH float32, reset bool) {
	r := 0
	if reset {
		r = 1
	}
	C.nexInputRect(C.uintptr_t(window), C.double(x), C.double(y), C.double(width), C.double(height), C.double(canvasW), C.double(canvasH), C.int(r))
}

func ReducedMotion() bool { return C.nexReducedMotion() != 0 }
