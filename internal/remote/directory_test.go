package remote

import (
	"io"
	"strings"
	"testing"
)

type byteReader struct{ io.Reader }

func (r byteReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:1]) }
func TestTerminalStartupPreservesOutput(t *testing.T) {
	marker := "\x1eNexShell:fixture:"
	for _, reader := range []io.Reader{strings.NewReader("欢迎\r\n" + marker + "123\x1f$ "), byteReader{strings.NewReader("欢迎\r\n" + marker + "123\x1f$ ")}} {
		pid, out, err := readTerminalPID(reader, marker)
		if err != nil || pid != 123 {
			t.Fatal(pid, err)
		}
		b, err := io.ReadAll(out)
		if err != nil || string(b) != "欢迎\r\n$ " {
			t.Fatalf("lost startup output %q %v", b, err)
		}
	}
	if _, _, err := readTerminalPID(strings.NewReader(marker+"invalid\x1f"), marker); err == nil {
		t.Fatal("invalid PID accepted")
	}
}
