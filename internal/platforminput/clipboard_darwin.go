//go:build darwin && !ci

package platforminput

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
char* nexClipboardFiles(void);
*/
import "C"
import (
	"encoding/json"
	"unsafe"
)

func ClipboardFiles() ([]string, error) {
	data := C.nexClipboardFiles()
	if data == nil {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(data))
	var paths []string
	err := json.Unmarshal([]byte(C.GoString(data)), &paths)
	return paths, err
}
