package transfer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/transfer/zmodem"
	"io"
	"sync"
	"time"
)

type Hooks struct {
	UploadFiles       func(context.Context) ([]string, error)
	DownloadDirectory func(context.Context) (string, error)
	Progress          zmodem.Progress
	State             func(bool, error)
}
type packet struct {
	b   []byte
	err error
}
type packetReader struct {
	ctx      context.Context
	packets  <-chan packet
	buffered []byte
	err      error
}

func (r *packetReader) Read(b []byte) (int, error) {
	for len(r.buffered) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		select {
		case p, ok := <-r.packets:
			if !ok {
				return 0, io.EOF
			}
			r.buffered = p.b
			r.err = p.err
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		}
	}
	n := copy(b, r.buffered)
	r.buffered = r.buffered[n:]
	return n, nil
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(b)
}

// Router separates terminal output and negotiated ZMODEM frames on the same SSH channel.
type Router struct {
	output         *io.PipeReader
	writer         *lockedWriter
	ctx            context.Context
	cancel         context.CancelFunc
	hooks          Hooks
	mu             sync.Mutex
	transferCancel context.CancelFunc
	transferring   bool
}

func NewRouter(ctx context.Context, input io.Writer, output io.Reader, hooks Hooks) *Router {
	ctx, cancel := context.WithCancel(ctx)
	r, w := io.Pipe()
	router := &Router{output: r, writer: &lockedWriter{w: input}, ctx: ctx, cancel: cancel, hooks: hooks}
	packets := make(chan packet, 8)
	go func() {
		defer close(packets)
		buf := make([]byte, 32768)
		for {
			n, e := output.Read(buf)
			p := packet{append([]byte{}, buf[:n]...), e}
			select {
			case packets <- p:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
	}()
	go router.pump(w, packets)
	return router
}
func (r *Router) Read(b []byte) (int, error) { return r.output.Read(b) }
func (r *Router) Write(b []byte) (int, error) {
	r.mu.Lock()
	active := r.transferring
	cancel := r.transferCancel
	r.mu.Unlock()
	if active {
		if bytes.Contains(b, []byte{3}) && cancel != nil {
			cancel()
		}
		return len(b), nil
	}
	return r.writer.Write(b)
}
func (r *Router) CancelTransfer() {
	r.mu.Lock()
	cancel := r.transferCancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (r *Router) Close() error { r.cancel(); return r.output.Close() }
func (r *Router) pump(display *io.PipeWriter, packets <-chan packet) {
	defer display.Close()
	source := &packetReader{ctx: r.ctx, packets: packets}
	reader := bufio.NewReader(source)
	magic := []byte{'*', '*', 0x18, 'B'}
	var pending, normal []byte
	flush := func() error {
		if len(normal) == 0 {
			return nil
		}
		_, e := display.Write(normal)
		normal = normal[:0]
		return e
	}
	for {
		b, e := reader.ReadByte()
		if e != nil {
			normal = append(normal, pending...)
			_ = flush()
			return
		}
		pending = append(pending, b)
		for len(pending) > 0 && !bytes.HasPrefix(magic, pending) {
			normal = append(normal, pending[0])
			pending = pending[1:]
		}
		if bytes.Equal(pending, magic) {
			if e = flush(); e != nil {
				return
			}
			pending = nil
			raw := make([]byte, 14)
			if _, e = io.ReadFull(reader, raw); e != nil {
				return
			}
			decoded := make([]byte, 7)
			if _, e = hex.Decode(decoded, raw); e != nil || zmodem.CRC16(decoded[:5]) != binary.BigEndian.Uint16(decoded[5:]) {
				normal = append(normal, magic...)
				normal = append(normal, raw...)
				continue
			}
			h := zmodem.Header{Type: decoded[0]}
			copy(h.Data[:], decoded[1:5])
			for i := 0; i < 2; i++ {
				v, e := reader.ReadByte()
				if e != nil {
					return
				}
				if v != '\r' && v&0x7f != '\n' {
					reader.UnreadByte()
					break
				}
			}
			if h.Type != zmodem.ZRQINIT && h.Type != zmodem.ZRINIT {
				continue
			}
			transferCtx, cancel := context.WithTimeout(r.ctx, 30*time.Minute)
			source.ctx = transferCtx
			r.mu.Lock()
			r.transferring = true
			r.transferCancel = cancel
			r.mu.Unlock()
			if r.hooks.State != nil {
				r.hooks.State(true, nil)
			}
			codec := zmodem.New(reader, r.writer)
			if h.Type == zmodem.ZRINIT {
				if r.hooks.UploadFiles == nil {
					e = errors.New("上传选择器未配置")
				} else {
					var files []string
					files, e = r.hooks.UploadFiles(transferCtx)
					if e == nil {
						e = codec.Send(transferCtx, files, &h, r.hooks.Progress)
					}
				}
			} else {
				if r.hooks.DownloadDirectory == nil {
					e = errors.New("下载目录未配置")
				} else {
					var dir string
					dir, e = r.hooks.DownloadDirectory(transferCtx)
					if e == nil {
						e = codec.Receive(transferCtx, dir, &h, r.hooks.Progress)
					}
				}
			}
			if e != nil {
				codec.Cancel()
			}
			cancel()
			source.ctx = r.ctx
			r.mu.Lock()
			r.transferring = false
			r.transferCancel = nil
			r.mu.Unlock()
			if r.hooks.State != nil {
				r.hooks.State(false, e)
			}
		}
		if reader.Buffered() == 0 || len(normal) >= 32768 {
			if e = flush(); e != nil {
				return
			}
		}
	}
}
