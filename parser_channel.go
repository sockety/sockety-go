package sockety

import (
	"errors"
	"github.com/google/uuid"
	"io"
)

// TODO: Include reference to connection
// TODO: Consider single byte array instead of fields list - it should decrease size (i.e. for simple "ping" action - from 80B to ~40B)
type message struct {
	stream          io.Reader
	id              uuid.UUID
	action          string
	dataSize        uint64
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
	return m.dataSize
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
	messageReader *messageReader
}

func newParserChannel() *parserChannel {
	return &parserChannel{
		messageReader: newMessageReader(),
	}
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

func (p *parserChannel) processMessage(b BufferedReader) (Message, error) {
	done, err := p.messageReader.Get(p.message, b)
	if err != nil && err != ErrNotReady {
		return nil, err
	} else if !done {
		return nil, nil
	}
	m := p.message
	// TODO: It should get rid of message only when all sub-data has been processed too
	p.message = nil
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
