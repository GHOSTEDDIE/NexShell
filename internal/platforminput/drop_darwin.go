//go:build darwin && !ci

package platforminput

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
int nexDropPosition(uintptr_t handle, double* x, double* y);
*/
import "C"

func DropPosition(handle uintptr) (float64, float64, bool) {
	var x, y C.double
	ok := C.nexDropPosition(C.uintptr_t(handle), &x, &y)
	return float64(x), float64(y), ok != 0
}
