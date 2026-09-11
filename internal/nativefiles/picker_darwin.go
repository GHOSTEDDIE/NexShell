//go:build darwin && !ci

package nativefiles

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
#include <stdlib.h>
void nexChooseFiles(uintptr_t token, uintptr_t parent, int mode, const char* title, const char* filename, const char* filters);
void nexCancelFiles(uintptr_t token);
*/
import "C"

import (
	"context"
	"encoding/json"
	"runtime/cgo"
	"unsafe"
)

type selection struct {
	paths []string
	err   error
}

//export nexFilesSelected
func nexFilesSelected(token C.uintptr_t, data *C.char) {
	handle := cgo.Handle(token)
	var result selection
	if data == nil {
		result.err = ErrCanceled
	} else {
		result.err = json.Unmarshal([]byte(C.GoString(data)), &result.paths)
	}
	handle.Value().(chan selection) <- result
}

func Choose(ctx context.Context, r Request) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(chan selection, 1)
	handle := cgo.NewHandle(result)
	defer handle.Delete()
	parent, _ := r.Parent.(uintptr)
	filters, _ := json.Marshal(r.Patterns)
	title, filename, patterns := C.CString(r.Title), C.CString(r.Filename), C.CString(string(filters))
	C.nexChooseFiles(C.uintptr_t(handle), C.uintptr_t(parent), C.int(r.Mode), title, filename, patterns)
	C.free(unsafe.Pointer(title))
	C.free(unsafe.Pointer(filename))
	C.free(unsafe.Pointer(patterns))
	select {
	case picked := <-result:
		return picked.paths, picked.err
	case <-ctx.Done():
		C.nexCancelFiles(C.uintptr_t(handle))
		<-result // Keep the Go callback handle alive until AppKit has dismissed.
		return nil, ctx.Err()
	}
}
