package terminal

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

// Network backpressure must never hold the emulator's state lock. This pipe
// queues encoded input without waiting for SSH. An unresponsive peer cannot
// cause unlimited accumulation: overflow terminates input with an explicit error.
const maxPendingInput = 1024 * 1024

var errInputBacklog = errors.New("终端输入积压超过 1 MiB，输入已停止，请重新连接")

type inputBuffer struct {
	mu    sync.Mutex
	ready *sync.Cond
	data  bytes.Buffer
	err   error
}

func newInputBuffer() *inputBuffer {
	p := &inputBuffer{}
	p.ready = sync.NewCond(&p.mu)
	return p
}
func (p *inputBuffer) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(b) == 0 {
		return 0, nil
	}
	for p.data.Len() == 0 && p.err == nil {
		p.ready.Wait()
	}
	if p.err != nil {
		return 0, p.err
	}
	return p.data.Read(b)
}
func (p *inputBuffer) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return 0, p.err
	}
	if len(b) > maxPendingInput-p.data.Len() {
		p.fail(errInputBacklog)
		return 0, p.err
	}
	n, err := p.data.Write(b)
	p.ready.Signal()
	return n, err
}
func (p *inputBuffer) fail(err error) {
	if p.err != nil {
		return
	}
	p.err = err
	p.data = bytes.Buffer{}
	p.ready.Broadcast()
}
func (p *inputBuffer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fail(io.EOF)
	return nil
}
func (p *inputBuffer) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err == io.EOF {
		return io.ErrClosedPipe
	}
	return p.err
}
