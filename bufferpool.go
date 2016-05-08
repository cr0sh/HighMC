package highmc

import "bytes"

// Pool is a default buffer pool for server.
var Pool = make(BufferPool, 1024)

// BufferPool is a type to recycle bytes.Buffer objects for reducing GC throughput.
type BufferPool chan *bytes.Buffer

// NewBuffer picks a recycled bytes.Buffer from pool.
// If pool is empty, NewBuffer creates new one.
// set bs to nil if you want empty buffer, without any initial values.
func (pool BufferPool) NewBuffer(bs []byte) (buf *bytes.Buffer) {
	select {
	case buf = <-pool:
	default: // pool is empty
		buf = new(bytes.Buffer)
	}
	buf.Write(bs)
	return
}

// Recycle resets and puts the buffer into the pool.
func (pool BufferPool) Recycle(buf *bytes.Buffer) {
	buf.Reset()
	select {
	case pool <- buf:
	default:
	}
}
