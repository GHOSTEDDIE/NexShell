package zmodem

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCRCAndHeaderGolden(t *testing.T) {
	if CRC16([]byte("123456789")) != 0x31c3 {
		t.Fatal("CRC16 standard check failed")
	}
	var b bytes.Buffer
	c := New(&b, &b)
	h := Header{Type: ZRINIT, Data: [4]byte{0, 0, 0, 0x23}}
	if e := c.WriteHeader(h); e != nil {
		t.Fatal(e)
	}
	if b.String() != "**\x18B0100000023be50\r\n\x11" {
		t.Fatalf("wire header %q", b.String())
	}
	got, e := c.ReadHeader()
	if e != nil || got.Type != h.Type || got.Data != h.Data {
		t.Fatal(got, e)
	}
}
func TestDataAllBytesBothCRCs(t *testing.T) {
	for _, crc32 := range []bool{false, true} {
		var b bytes.Buffer
		c := New(&b, &b)
		data := make([]byte, 256)
		for i := range data {
			data[i] = byte(i)
		}
		if e := c.WriteData(data, ZCRCW, crc32); e != nil {
			t.Fatal(e)
		}
		got, end, e := c.ReadData(crc32)
		if e != nil || end != ZCRCW || !bytes.Equal(data, got) {
			t.Fatal("binary escaping roundtrip", e)
		}
		var corrupt bytes.Buffer
		c = New(&corrupt, &corrupt)
		c.WriteData([]byte("test"), ZCRCE, crc32)
		raw := append([]byte{}, corrupt.Bytes()...)
		raw[0] = 'b'
		if _, _, e = New(bytes.NewReader(raw), io.Discard).ReadData(crc32); e == nil {
			t.Fatal("corrupt packet accepted")
		}
	}
}
func TestSendReceiveMultipleFilesAndResume(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	files := []string{filepath.Join(src, "中文.bin"), filepath.Join(src, "empty")}
	data := bytes.Repeat([]byte{0, 0x18, 0xff, 'x', 0x11, 0x13}, 5000)
	os.WriteFile(files[0], data, 0600)
	os.WriteFile(files[1], nil, 0600)
	os.WriteFile(filepath.Join(dst, "中文.bin"), data[:1000], 0600)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	deadline := time.Now().Add(10 * time.Second)
	a.SetDeadline(deadline)
	b.SetDeadline(deadline)
	ch := make(chan error, 1)
	go func() { ch <- New(b, b).Receive(context.Background(), dst, nil, nil) }()
	if e := New(a, a).Send(context.Background(), files, nil, nil); e != nil {
		t.Fatal(e)
	}
	if e := <-ch; e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(filepath.Join(dst, "中文.bin"))
	if e != nil || !bytes.Equal(data, got) {
		t.Fatal("received content mismatch", e)
	}
	if info, e := os.Stat(filepath.Join(dst, "empty")); e != nil || info.Size() != 0 {
		t.Fatal("empty file failed", e)
	}
}
func TestRejectTraversal(t *testing.T) {
	for _, name := range []string{"../key", "a/b", `a\b`, ".", "..", ""} {
		if safeName(name) {
			t.Errorf("unsafe name %q", name)
		}
	}
}
func FuzzFrameDecoder(f *testing.F) {
	f.Add([]byte("**\x18B0100000023be50\r\n\x11"))
	f.Add([]byte{24, 24, 24, 24, 24})
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = New(bytes.NewReader(b), io.Discard).ReadHeader() })
}
