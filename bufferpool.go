package highmc

import (
	"bytes"
	"sync"
)

// Pool is a default buffer pool for server.
var Pool = NewBufferPool()

// BufferPool is a wrapper struct for sync.Pool
type BufferPool struct {
	*sync.Pool
}

// NewBufferPool returns new BufferPool struct.
func NewBufferPool() BufferPool {
	p := BufferPool{new(sync.Pool)}
	p.New = func() interface{} {
		return new(bytes.Buffer)
	}
	return p
}

// NewBuffer picks a recycled bytes.Buffer from pool.
// If pool is empty, NewBuffer creates new one.
// set bs to nil if you want empty buffer, without any initial values.
func (pool BufferPool) NewBuffer(bs []byte) (buf *bytes.Buffer) {
	buf = pool.Get().(*bytes.Buffer)
	buf.Write(bs)
	return
}

// Recycle resets and puts the buffer into the pool.
func (pool BufferPool) Recycle(buf *bytes.Buffer) {
	buf.Reset()
	pool.Put(buf)
}
