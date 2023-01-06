package sockety

import (
	"errors"
	"github.com/sockety/sockety-go/internal/buffer_pool"
	"github.com/valyala/bytebufferpool"
	"io"
	"sync"
	"sync/atomic"
)

// TODO: Consider maximum buffer size, so 'push' could have backpressure?
type messageData struct {
	buf       []*bytebufferpool.ByteBuffer
	bufOffset uint64
	wrote     uint64
	size      uint64
	mu        sync.Mutex
	err       error
	ch        chan struct{}
}

type MessageData interface {
	io.Reader
	Received() uint64
	Size() uint64
	Done() bool
}

func newMessageData(size uint64) *messageData {
	return &messageData{
		size: size,
		ch:   make(chan struct{}),
	}
}

func (m *messageData) push(p *bytebufferpool.ByteBuffer) error {
	m.mu.Lock()
	wrote := m.wrote + uint64(p.Len())
	if wrote > m.size {
		m.err = errors.New("malformed size")
		close(m.ch)
		m.mu.Unlock()
		return m.err
	}
	m.buf = append(m.buf, p)
	atomic.SwapUint64(&m.wrote, wrote)
	if m.wrote == m.size {
		close(m.ch)
	} else {
		putOptional(m.ch, struct{}{})
	}
	m.mu.Unlock()
	return nil
}

func (m *messageData) Read(p []byte) (int, error) {
	size := uint64(len(p))

	for {
		m.mu.Lock()

		if m.err != nil {
			err := m.err
			m.mu.Unlock()
			return 0, err
		} else if len(m.buf) == 0 {
			if m.size == m.wrote {
				m.mu.Unlock()
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

func (m *messageData) Size() uint64 {
	return m.size
}

func (m *messageData) Received() uint64 {
	return atomic.LoadUint64(&m.wrote)
}

func (m *messageData) Done() bool {
	return m.Received() == m.size
}

var emptyMessageData = newMessageData(0)
