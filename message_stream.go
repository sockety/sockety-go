package sockety

import (
	"github.com/sockety/sockety-go/internal/buffer_pool"
	"github.com/valyala/bytebufferpool"
	"io"
	"sync"
	"sync/atomic"
)

// TODO: Consider maximum buffer size, so 'push' could have backpressure?
type messageStream struct {
	buf       []*bytebufferpool.ByteBuffer
	bufOffset uint64
	wrote     uint64
	mu        sync.Mutex
	done      uint32
	ch        chan struct{}
}

type MessageStream interface {
	io.Reader
	Received() uint64
	Done() bool
}

func newMessageStream() *messageStream {
	return &messageStream{
		ch: make(chan struct{}),
	}
}

func (m *messageStream) push(p *bytebufferpool.ByteBuffer) error {
	m.mu.Lock()
	m.buf = append(m.buf, p)
	atomic.SwapUint64(&m.wrote, m.wrote+uint64(p.Len()))
	putOptional(m.ch, struct{}{})
	m.mu.Unlock()
	return nil
}

func (m *messageStream) close() error {
	if atomic.SwapUint32(&m.done, 1) == 0 {
		close(m.ch)
	}
	return nil
}

func (m *messageStream) Read(p []byte) (int, error) {
	size := uint64(len(p))

	for {
		m.mu.Lock()

		if len(m.buf) == 0 {
			if m.Done() {
				return 0, io.EOF
			}

			go func() {
				m.mu.Unlock()
			}()
			<-m.ch
			continue
		}

		buf := m.buf[0]
		if uint64(len(buf.B))-m.bufOffset > size {
			copy(p, buf.B[m.bufOffset:])
			m.bufOffset += size
			m.mu.Unlock()
			return len(p), nil
		} else {
			copy(p, buf.B[m.bufOffset:])

			bytes := uint64(len(buf.B)) - m.bufOffset
			m.bufOffset = 0
			m.buf = m.buf[1:]
			m.mu.Unlock()
			buffer_pool.Release(buf)
			return int(bytes), nil
		}
	}
}

func (m *messageStream) Received() uint64 {
	return atomic.LoadUint64(&m.wrote)
}

func (m *messageStream) Done() bool {
	return atomic.LoadUint32(&m.done) == 1
}
