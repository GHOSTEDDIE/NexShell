package zmodem

import (
	"bytes"
	"context"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReceiveResumeWithRepeatedFileHeader(t *testing.T) {
	for _, tc := range []struct {
		name     string
		checksum uint32
		changed  bool
		success  bool
	}{
		{"matching", crc32.ChecksumIEEE([]byte("abc")), false, true},
		{"different-prefix", crc32.ChecksumIEEE([]byte("bad")), false, false},
		{"different-metadata", crc32.ChecksumIEEE([]byte("abc")), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "file.txt")
			if err := os.WriteFile(dest, []byte("abc"), 0600); err != nil {
				t.Fatal(err)
			}
			var wire bytes.Buffer
			c := New(nil, &wire)
			c.WriteHeader(Position(ZRQINIT, 0))
			meta := []byte("file.txt\x006 0 0 0 1 6\x00")
			c.WriteHeader(Position(ZFILE, 0))
			c.WriteData(meta, ZCRCW, false)
			c.WriteHeader(Position(ZFILE, 0))
			if tc.changed {
				meta = []byte("other.txt\x006 0 0 0 1 6\x00")
			}
			c.WriteData(meta, ZCRCW, false)
			c.WriteHeader(Position(ZCRC, tc.checksum))
			c.WriteHeader(Position(ZDATA, 3))
			c.WriteData([]byte("def"), ZCRCE, false)
			c.WriteHeader(Position(ZEOF, 6))
			c.WriteHeader(Position(ZFIN, 0))
			wire.WriteString("OO")
			var responses bytes.Buffer
			err := New(&wire, &responses).Receive(context.Background(), dir, nil, nil)
			if (err == nil) != tc.success {
				t.Fatalf("resume result: %v", err)
			}
			got, readErr := os.ReadFile(dest)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "abc"
			if tc.success {
				want = "abcdef"
			}
			if string(got) != want {
				t.Fatalf("destination=%q want=%q", got, want)
			}
			if tc.success {
				decoder := New(&responses, io.Discard)
				ready := 0
				for {
					header, err := decoder.ReadHeader()
					if err != nil {
						break
					}
					if header.Type == ZRINIT {
						ready++
					}
				}
				if ready != 2 {
					t.Fatalf("sent %d ready frames, want one startup and one file completion", ready)
				}
			}
		})
	}
}
