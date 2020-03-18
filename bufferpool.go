package uniq

import (
	"bytes"
	"sync"
)

// bufferPool wraps a *sync.Pool with Get and Put functions typed for byte
// buffers.
type bufferPool struct {
	underlying *sync.Pool
}

func newBufferPool() *bufferPool {
	p := new(bufferPool)
	p.underlying = &sync.Pool{
		New: func() interface{} { return new(bytes.Buffer) },
	}
	return p
}

// Get retrieves a byte buffer from p.underlying and resets it so that it is
// ready to use.
func (p *bufferPool) Get() *bytes.Buffer {
	buf := p.underlying.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (p *bufferPool) Put(buf *bytes.Buffer) {
	p.underlying.Put(buf)
}
