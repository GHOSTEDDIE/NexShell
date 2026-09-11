//go:build darwin && !ci

package nativeime

import (
	"github.com/go-gl/glfw/v3.4/glfw"
	"os"
	"runtime"
	"testing"
)

var contractCode, enters int
var committed string

func init() { runtime.LockOSThread() }
func TestMain(m *testing.M) {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	glfw.WindowHint(glfw.Visible, glfw.False)
	w, err := glfw.CreateWindow(500, 300, "NexShell IME test", nil, nil)
	if err != nil {
		panic(err)
	}
	w.SetCharCallback(func(_ *glfw.Window, r rune) { committed += string(r) })
	w.SetKeyCallback(func(_ *glfw.Window, k glfw.Key, _ int, a glfw.Action, _ glfw.ModifierKey) {
		if k == glfw.KeyEnter && a != glfw.Release {
			enters++
		}
	})
	contractCode = probe(uintptr(w.GetCocoaWindow()))
	w.Destroy()
	glfw.Terminate()
	os.Exit(m.Run())
}
func TestNativeCompositionContract(t *testing.T) {
	if contractCode != 0 {
		t.Errorf("native text-input contract failure bits: %d", contractCode)
	}
	if committed != "你好" {
		t.Errorf("committed text = %q", committed)
	}
	if enters != 1 {
		t.Errorf("composing Return leaked or normal Return lost: %d", enters)
	}
}
