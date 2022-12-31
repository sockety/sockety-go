package sockety

import (
	"github.com/google/uuid"
	"io"
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

//func (m *MessageDraft) Stream() MessageDraft {
//	m2 := m
//	m2.HasStream = true
//	return m2
//}
//
//func (m *MessageDraft) NoStream() MessageDraft {
//	m2 := m
//	m2.HasStream = false
//	return m2
//}

// Producer interface

func send(m MessageDraft, id uuid.UUID, w Writer, expectsResponse bool) error {
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
	channel, err := w.WriteAndObtainChannel(p)
	if err != nil {
		return err
	}

	// Write data
	// TODO: Make it async
	// TODO: Support splitting >uint32 size
	err = w.WriteAtChannel(
		channel,
		newPacket(packetDataBits, offset(len(m.Data))).Bytes(m.Data),
	)
	if err != nil {
		w.ReleaseChannel(channel)
		return err
	}

	// TODO: Build packets for files too

	// Release channel at the end
	w.ReleaseChannel(channel)

	return nil
}

func (m *MessageDraft) pass(writer Writer) error {
	return send(*m, uuid.New(), writer, false)
}

func (m *MessageDraft) create(writer Writer) Request {
	return &messageRequest{
		id: uuid.New(),
		w:  writer,
		m:  m,
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

// TODO: Consider if it makes sense at all
func (m *MessageDraft) RequestToAndSend(c Conn) error {
	if cc, ok := c.(*conn); ok {
		return send(*m, uuid.New(), cc.w, true)
	}
	panic("passed connection without internal producer's support")
	//return m.RequestTo(c).Send()
}

type messageRequest struct {
	id uuid.UUID
	m  *MessageDraft
	w  Writer
}

func (m *messageRequest) Id() uuid.UUID {
	return m.id
}

func (m *messageRequest) Send() error {
	return send(*m.m, m.id, m.w, true)
}

func (m *messageRequest) Stream() io.Writer {
	panic("not implemented")
	return nil
}

func (m *messageRequest) Initiated() <-chan struct{} {
	panic("not implemented")
	return createReadChannel(struct{}{})
}

func (m *messageRequest) Done() <-chan struct{} {
	panic("not implemented")
	return createReadChannel(struct{}{})
}
