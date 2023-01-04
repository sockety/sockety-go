package sockety

import (
	"errors"
	"github.com/sockety/sockety-go/internal/done_signal"
	"github.com/sockety/sockety-go/internal/fifo_mutex"
	"io"
)

type messageStreamWriter struct {
	channel ChannelID
	writer  Writer
	done    *done_signal.DoneSignal
	mu      *fifo_mutex.FIFOMutex
	err     error
}

// TODO: Handle Abort instruction?
func newMessageStreamWriter(channel ChannelID, writer Writer) *messageStreamWriter {
	w := &messageStreamWriter{
		channel: channel,
		writer:  writer,
		mu:      fifo_mutex.New(1),
		done:    done_signal.New(),
	}
	w.mu.Lock() // Stream should be initially locked
	return w
}

func (m *messageStreamWriter) Done() <-chan struct{} {
	return m.done.Get()
}

func (m *messageStreamWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	if m.done.Load() {
		m.mu.Unlock()
		return 0, errors.New("stream is already closed")
	}
	// TODO: Support splitting >uint32 size
	err := m.writer.WriteExternalWithSignatureAtChannel(m.channel, packetStreamBits, p)
	if err != nil {
		m.err = err
		m.done.Close()
		m.mu.Unlock()
		return 0, err
	}
	m.mu.Unlock()
	return len(p), nil
}

func (m *messageStreamWriter) Close() error {
	m.mu.Lock()
	if m.done.Load() {
		m.mu.Unlock()
		return nil
	}
	err := m.writer.WriteRawAtChannel(m.channel, []byte{packetStreamEndBits})
	if err != nil {
		m.err = err
		m.mu.Unlock()
		return err
	}
	m.done.Close()
	m.mu.Unlock()
	return nil
}

func (m *messageStreamWriter) Err() error {
	m.mu.Lock()
	err := m.err
	m.mu.Unlock()
	return err
}

// TODO: Optimize
func (m *messageStreamWriter) ReadFrom(r io.Reader) (int64, error) {
	written, err := io.Copy(m, r)
	if err == nil {
		err = m.Close()
	}
	return written, err
}
