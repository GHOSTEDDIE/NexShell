package remote

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

func localSFTPClient(t testing.TB) *sftp.Client {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	server, err := sftp.NewServer(serverConn)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client
}

func TestTransferWithClientPreservesUploadDownloadAndResume(t *testing.T) {
	client := localSFTPClient(t)
	data := bytes.Repeat([]byte("correct-data-中文\x00"), 1024)
	local := filepath.Join(t.TempDir(), "source")
	remotePath := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(local, data, 0600); err != nil {
		t.Fatal(err)
	}
	var progress [][2]int64
	callback := func(n, total int64) { progress = append(progress, [2]int64{n, total}) }
	if err := transferWithClient(context.Background(), client, local, remotePath, true, false, true, callback); err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 || progress[0] != [2]int64{0, int64(len(data))} || progress[len(progress)-1] != [2]int64{int64(len(data)), int64(len(data))} {
		t.Fatalf("upload progress = %v", progress)
	}
	got, err := os.ReadFile(remotePath)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("uploaded content mismatch: %v", err)
	}
	download := filepath.Join(t.TempDir(), "download")
	if err = transferWithClient(context.Background(), client, download, remotePath, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(download)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("downloaded content mismatch: %v", err)
	}
	if err = os.WriteFile(remotePath, data[:200], 0600); err != nil {
		t.Fatal(err)
	}
	progress = nil
	if err = transferWithClient(context.Background(), client, local, remotePath, true, true, false, callback); err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 || progress[0] != [2]int64{200, int64(len(data))} {
		t.Fatalf("resume progress = %v", progress)
	}
	got, err = os.ReadFile(remotePath)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("resumed content mismatch: %v", err)
	}
	corrupt := append([]byte(nil), data[:200]...)
	corrupt[0] ^= 0xff
	if err = os.WriteFile(remotePath, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err = transferWithClient(context.Background(), client, local, remotePath, true, true, false, nil); err == nil {
		t.Fatal("incompatible resume was accepted")
	}
	got, err = os.ReadFile(remotePath)
	if err != nil || !bytes.Equal(got, corrupt) {
		t.Fatalf("incompatible resume modified target: %v", err)
	}
}

func TestTransferRejectsSourceChangedDuringUpload(t *testing.T) {
	client := localSFTPClient(t)
	data := bytes.Repeat([]byte("old-content-"), 1000)
	local := filepath.Join(t.TempDir(), "source")
	remotePath := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(local, data, 0600); err != nil {
		t.Fatal(err)
	}
	changed := false
	err := transferWithClient(context.Background(), client, local, remotePath, true, false, true, func(n, total int64) {
		if n == total && !changed {
			changed = true
			if writeErr := os.WriteFile(local, bytes.Repeat([]byte("new-content-"), 1000), 0600); writeErr != nil {
				t.Error(writeErr)
			}
		}
	})
	if err == nil {
		t.Fatal("changed source was accepted")
	}
}

func BenchmarkTransferUploadAndVerify(b *testing.B) {
	client := localSFTPClient(b)
	data := bytes.Repeat([]byte("benchmark-data-"), 1<<19)
	local := filepath.Join(b.TempDir(), "source")
	remotePath := filepath.Join(b.TempDir(), "target")
	if err := os.WriteFile(local, data, 0600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := transferWithClient(context.Background(), client, local, remotePath, true, false, false, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLocalSequentialUploadAndVerify(b *testing.B) {
	client := localSFTPClient(b)
	data := bytes.Repeat([]byte("benchmark-data-"), 1<<19)
	local := filepath.Join(b.TempDir(), "source")
	remotePath := filepath.Join(b.TempDir(), "target")
	if err := os.WriteFile(local, data, 0600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := sequentialUploadAndVerify(client, local, remotePath); err != nil {
			b.Fatal(err)
		}
	}
}

func sequentialUploadAndVerify(client *sftp.Client, local, remotePath string) error {
	source, err := os.Open(local)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := client.OpenFile(remotePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	_, err = io.Copy(target, source)
	closeErr := target.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	sourceHash := sha256.New()
	if _, err = io.Copy(sourceHash, source); err != nil {
		return err
	}
	verify, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer verify.Close()
	targetHash := sha256.New()
	if _, err = io.Copy(targetHash, verify); err != nil {
		return err
	}
	if !equalHash(sourceHash.Sum(nil), targetHash.Sum(nil)) {
		return errors.New("hash mismatch")
	}
	return nil
}

type delayedReader struct {
	io.ReaderAt
	delay time.Duration
}

func (r delayedReader) ReadAt(p []byte, off int64) (int, error) {
	time.Sleep(r.delay)
	return r.ReaderAt.ReadAt(p, off)
}

type delayedWriter struct {
	io.WriterAt
	delay time.Duration
}

func (w delayedWriter) WriteAt(p []byte, off int64) (int, error) {
	time.Sleep(w.delay)
	return w.WriterAt.WriteAt(p, off)
}

type delayedFileReader struct {
	base  sftp.FileReader
	delay time.Duration
}

func (r delayedFileReader) Fileread(request *sftp.Request) (io.ReaderAt, error) {
	file, err := r.base.Fileread(request)
	if err != nil {
		return nil, err
	}
	return delayedReader{file, r.delay}, nil
}

type delayedFileWriter struct {
	base  sftp.FileWriter
	delay time.Duration
}

func (w delayedFileWriter) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	file, err := w.base.Filewrite(request)
	if err != nil {
		return nil, err
	}
	return delayedWriter{file, w.delay}, nil
}

func delayedSFTPClient(b *testing.B, delay time.Duration) *sftp.Client {
	b.Helper()
	clientConn, serverConn := net.Pipe()
	handlers := sftp.InMemHandler()
	handlers.FileGet = delayedFileReader{handlers.FileGet, delay}
	handlers.FilePut = delayedFileWriter{handlers.FilePut, delay}
	server := sftp.NewRequestServer(serverConn, handlers)
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		server.Close()
		b.Fatal(err)
	}
	b.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client
}

func BenchmarkDelayedSFTPUploadAndVerify(b *testing.B) {
	for _, adaptive := range []bool{false, true} {
		name := "sequential"
		if adaptive {
			name = "adaptive"
		}
		b.Run(name, func(b *testing.B) {
			client := delayedSFTPClient(b, 2*time.Millisecond)
			data := bytes.Repeat([]byte("slow-link-data-"), 1<<18)
			local := filepath.Join(b.TempDir(), "source")
			if err := os.WriteFile(local, data, 0600); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if adaptive {
					if err := transferWithClient(context.Background(), client, local, "/payload", true, false, false, nil); err != nil {
						b.Fatal(err)
					}
					continue
				}
				if err := sequentialUploadAndVerify(client, local, "/payload"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
