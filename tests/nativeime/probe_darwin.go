//go:build darwin && !ci

package nativeime

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
int imeProbe(uintptr_t window);
*/
import "C"

func probe(window uintptr) int { return int(C.imeProbe(C.uintptr_t(window))) }
