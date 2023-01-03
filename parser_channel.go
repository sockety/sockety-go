package sockety

import (
	"errors"
	"github.com/google/uuid"
	"github.com/sockety/sockety-go/internal/buffer_pool"
	"io"
)

// TODO: Include reference to connection
// TODO: Consider single byte array instead of fields list - it should decrease size (i.e. for simple "ping" action - from 80B to ~40B)
type message struct {
	stream          io.Reader
	data            MessageData
	id              uuid.UUID
	action          string
	totalFilesSize  uint64
	filesCount      uint32
	expectsResponse bool
}

func (m *message) Id() uuid.UUID {
	return m.id
}

func (m *message) Action() string {
	return m.action
}

func (m *message) DataSize() uint64 {
	return m.data.Size()
}

func (m *message) Data() MessageData {
	return m.data
}

func (m *message) TotalFilesSize() uint64 {
	return m.totalFilesSize
}

func (m *message) FilesCount() uint32 {
	return m.filesCount
}

func (m *message) Stream() io.Reader {
	return m.stream
}

func (m *message) ExpectsResponse() bool {
	return m.expectsResponse
}

type response struct {
	expectsResponse bool
	stream          io.Reader
}

type ParserChannelOptions struct {
}

type ParserChannelResult interface {
	message | Response
}

type parserChannel struct {
	message       *message
	response      *response
	data          *messageData
	messageReader *messageReader
}

func newParserChannel() *parserChannel {
	return &parserChannel{
		messageReader: newMessageReader(),
	}
}

func (p *parserChannel) DataDone() bool {
	return p.data == nil || (*p.data).Done()
}

func (p *parserChannel) Idle() bool {
	return p.message == nil
}

func (p *parserChannel) InitMessage(expectsResponse bool, hasStream bool) error {
	if !p.Idle() {
		return errors.New("channel is already processing")
	}
	p.message = &message{
		expectsResponse: expectsResponse,
		//stream:       hasStream,
	}
	return nil
}

func (p *parserChannel) InitResponse(expectsResponse bool, hasStream bool) error {
	if !p.Idle() {
		return errors.New("channel is already processing")
	}
	p.response = &response{
		expectsResponse: expectsResponse,
		//stream:       hasStream,
	}
	return nil
}

func (p *parserChannel) maybeEnd() {
	// TODO: It should get rid of message only when all sub-data has been processed too
	if p.DataDone() {
		p.message = nil
		p.data = nil
	}
}

func (p *parserChannel) processMessage(b BufferedReader) (Message, error) {
	done, err := p.messageReader.Get(p.message, b)
	if err != nil && err != ErrNotReady {
		return nil, err
	} else if !done {
		return nil, nil
	}
	m := p.message

	if m.data.Size() > 0 {
		p.data = m.data.(*messageData)
	}

	p.maybeEnd()
	return m, nil
}

func (p *parserChannel) processResponse(b BufferedReader) (Response, error) {
	return nil, errors.New("not implemented")
}

func (p *parserChannel) Process(b BufferedReader) (ParserChannelResult, error) {
	if p.message != nil {
		return p.processMessage(b)
	} else if p.response != nil {
		return p.processResponse(b)
	} else {
		return nil, errors.New("channel is not processing")
	}
}

func (p *parserChannel) ProcessData(b BufferedReader) error {
	if p.DataDone() {
		return errors.New("channel does not expect any Data")
	}

	for {
		step := b.Len()

		if step == 0 {
			p.maybeEnd()
			return nil
		}

		// TODO: make it configurable?
		if step > 65_536 {
			step = 65_536
		}

		buf := buffer_pool.ObtainUnsafe(step)
		_, err := b.Read(buf.B) // TODO: Write directly to p.data?
		if err != nil {
			return err
		}
		err = p.data.push(buf)
		if err != nil {
			return err
		}
	}
}
