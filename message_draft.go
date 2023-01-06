package sockety

import (
	"errors"
	"github.com/google/uuid"
	"github.com/sockety/sockety-go/internal/done_signal"
	"io"
	"sync"
)

// TODO: Consider with pointers

type MessageDraft struct {
	HasStream bool
	Action    string
	Data      []byte
	//Files []FileTransfer // TODO
}

// Constructor

func NewMessageDraft(action string) *MessageDraft {
	return &MessageDraft{
		Action: action,
	}
}

// Builder pattern

func (m *MessageDraft) RawData(data []byte) *MessageDraft {
	m2 := *m
	mCopy := m2
	mCopy.Data = data
	return &mCopy
}

func (m *MessageDraft) Stream() *MessageDraft {
	m2 := *m
	mCopy := m2
	mCopy.HasStream = true
	return &mCopy
}

func (m *MessageDraft) NoStream() *MessageDraft {
	m2 := *m
	mCopy := m2
	mCopy.HasStream = true
	return &mCopy
}

// Producer interface

func send(m MessageDraft, id uuid.UUID, w Writer, stream *messageStreamWriter, providedChannel *ChannelID, expectsResponse bool) error {
	// TODO: Cache calculations (?)
	// Estimate action size
	actionSize := len(m.Action)
	actionSizeBytes := getNameSizeBytes(actionSize)

	// Estimate data size
	dataSize := len(m.Data)
	dataSizeBytes := getDataSizeBytes(dataSize)

	// Estimate files count & size
	filesCount := 0
	filesCountBytes := getFilesCountBytes(filesCount)
	totalFilesSize := 0
	totalFilesSizeBytes := getFilesSizeBytes(totalFilesSize, filesCount)

	// Build message flags
	flags := getNameSizeFlag(actionSizeBytes) | getDataSizeFlag(dataSizeBytes) | getFilesCountFlag(filesCountBytes) | getFilesSizeFlag(totalFilesSizeBytes)

	// Estimate "Message" packet size
	messageSize := 17 + offset(actionSizeBytes) + offset(actionSize) + offset(dataSizeBytes) + offset(filesCountBytes) + offset(totalFilesSizeBytes)

	// Build signature
	signature := uint8(packetMessageBits)
	if expectsResponse {
		signature |= expectsResponseBits
	}
	if m.HasStream {
		signature |= hasStreamBits
	}

	// Build "Message" packet
	p := newPacket(signature, messageSize)
	p = p.Uint8(flags)
	p = p.UUID(id)
	p = p.Uint(Uint48(actionSize), actionSizeBytes)
	p = p.String(m.Action)
	if dataSizeBytes != 0 {
		p = p.Uint(Uint48(dataSize), dataSizeBytes)
	}
	if filesCount > 0 {
		p = p.Uint(Uint48(filesCount), filesCountBytes)
		p = p.Uint(Uint48(totalFilesSize), totalFilesSizeBytes)
	}

	// Fast-track when there is no stream, data and files
	if dataSizeBytes == 0 && filesCount == 0 && !m.HasStream {
		return w.Write(p)
	}

	// Write message packet
	var channel ChannelID
	if providedChannel == nil {
		channel = w.ObtainChannel()
	} else {
		channel = *providedChannel
	}
	err := w.WriteAtChannel(channel, p)
	if err != nil {
		return err
	}

	// Prepare wait group for all async message parts
	wg := sync.WaitGroup{}
	var finalErr error

	// Write data
	if dataSizeBytes > 0 {
		wg.Add(1)

		// TODO: Support splitting >uint32 size
		// TODO: Consider worker pools
		go func() {
			err = w.WriteExternalWithSignatureAtChannel(channel, packetDataBits, m.Data)
			if err != nil {
				finalErr = err
			}
			wg.Done()
		}()
	}

	// Write stream
	if m.HasStream {
		// Fast-track when there is no stream, but needs to be simulated
		if stream == nil {
			err = w.WriteRawAtChannel(channel, []byte{packetStreamEndBits})
			if err != nil {
				finalErr = err
			}
		} else {
			wg.Add(1)
			stream.mu.Unlock() // Start accepting data in Stream

			// TODO: Support splitting >uint32 size
			go func() {
				<-stream.Done()
				err = stream.Err()
				if err != nil {
					finalErr = err
				}
				wg.Done()
			}()
		}
	}

	// TODO: Build packets for files too

	// Wait for all parts sent and notify
	wg.Wait()
	w.ReleaseChannel(channel)

	return finalErr
}

func (m *MessageDraft) pass(writer Writer) error {
	return send(*m, uuid.New(), writer, nil, nil, false)
}

func (m *MessageDraft) create(writer Writer) Request {
	return &messageRequest{
		id:        uuid.New(),
		w:         writer,
		m:         m,
		initiated: done_signal.New(),
		done:      done_signal.New(),
	}
}

func (m *MessageDraft) RequestTo(c Conn) Request {
	if cc, ok := c.(*conn); ok {
		return m.create(cc.w)
	}
	panic("passed connection without internal producer's support")
}

func (m *MessageDraft) PassTo(c Conn) error {
	if cc, ok := c.(*conn); ok {
		return m.pass(cc.w)
	}
	panic("passed connection without internal producer's support")
}

type messageRequest struct {
	id        uuid.UUID
	m         *MessageDraft
	w         Writer
	mu        sync.Mutex
	initiated *done_signal.DoneSignal
	done      *done_signal.DoneSignal
	stream    *messageStreamWriter
}

func (m *messageRequest) Id() uuid.UUID {
	return m.id
}

func (m *messageRequest) Send() error {
	// Allow sending request only once
	if !m.mu.TryLock() {
		return errors.New("request is already sent")
	}

	// If stream is expected, obtain channel and prepare stream
	if m.m.HasStream {
		channel := m.w.ObtainChannel()
		m.stream = newMessageStreamWriter(channel, m.w)
		m.initiated.Close()
		err := send(*m.m, m.id, m.w, m.stream, &channel, true)
		m.done.Close()
		return err
	}

	// Otherwise, just send
	err := send(*m.m, m.id, m.w, nil, nil, true)
	m.done.Close()
	return err
}

func (m *messageRequest) Stream() io.WriteCloser {
	if m.initiated.Load() {
		return m.stream
	}
	<-m.initiated.Get()
	return m.stream
}

func (m *messageRequest) Initiated() <-chan struct{} {
	return m.initiated.Get()
}

func (m *messageRequest) Done() <-chan struct{} {
	return m.done.Get()
}
