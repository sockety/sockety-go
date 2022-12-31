package sockety

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/sockety/sockety-go/internal/done_signal"
	"io"
	"net"
	"strings"
	"time"
)

type ConnOptions struct {
	Channels         uint16        // default: max
	WriteChannels    uint16        // default: max
	BufferedMessages uint16        // default: 0
	ReadBufferSize   uint32        // default: 4096
	Timeout          time.Duration // default: 0 = none
}

type conn struct {
	// Components
	conn io.ReadWriter
	w    *writer
	p    *parser

	// State: basic
	ctx       context.Context
	ctxCancel context.CancelFunc

	// State: finishing connection
	readDone *done_signal.DoneSignal
	closed   *done_signal.DoneSignal
	done     *done_signal.DoneSignal

	// State: channels for exposing information about different types
	messages    chan Message
	fastReplies chan FastReply
	responses   chan Response
}

type Message interface {
	Id() uuid.UUID
	Action() string
	DataSize() uint64
	TotalFilesSize() uint64
	FilesCount() uint32
	ExpectsResponse() bool
	Stream() io.Reader
	Data() MessageData
	// TODO: Responses

	// TODO: Discard()
}

type Response interface {
}

type GoAway interface {
}

type Request interface {
	Id() uuid.UUID
	Send() error
	Initiated() <-chan struct{}
	Done() <-chan struct{}
}

type Producer interface {
	pass(w Writer) error
	// PassTo(c Conn) error // TODO: Consider
}

type ProducerWithRequest interface {
	create(w Writer) Request
	// RequestAt(c Conn) Request // TODO: Consider
	// RequestAtAndWaitForResponse(c Conn) Response // TODO: Consider
}

type Conn interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	Request(p ProducerWithRequest) Request
	Pass(p Producer) error
	// RequestAndWaitForResponse(p ProducerWithRequest) Response // TODO: later

	Close() error

	ReadDone() <-chan struct{}
	ConnectionClosed() <-chan struct{}
	Done() <-chan struct{}

	Messages() <-chan Message
	Responses() <-chan Response
	FastReplies() <-chan FastReply
}

// Factories

func NewConn(parentCtx context.Context, rw io.ReadWriter, options *ConnOptions) (Conn, error) {
	// Use default options
	if options == nil {
		options = &ConnOptions{}
	}

	// Read options
	readChannels := getDefault(options.Channels, MaxChannels)
	writeChannels := getDefault(options.WriteChannels, MaxChannels)
	readBufferSize := getDefault(options.ReadBufferSize, DefaultReadBufferSize)

	// Validate options
	if err := validateChannelsCount(readChannels); err != nil {
		return nil, err
	}
	if err := validateChannelsCount(writeChannels); err != nil {
		return nil, err
	}

	// Set up parser
	p := NewParser(rw, ParserOptions{
		Channels:   readChannels,
		BufferSize: readBufferSize,
	}).(*parser)

	// Write connection header
	_, err := rw.Write(createProtocolHeader(readChannels))
	if err != nil {
		return nil, err
	}

	// TODO: Read connection header
	header, err := p.ReadHeader()
	if err != nil {
		fmt.Println("reading conneciton header", err)
		return nil, err
	}
	if header.Channels < writeChannels {
		writeChannels = header.Channels
	}

	// Set up writer
	w := NewWriter(rw, WriterOptions{
		Channels: writeChannels,
	}).(*writer)

	// Set up channels for messages
	// TODO: Test if that couldn't be just make(chan Message, options.BufferedMessages)
	var messages chan Message
	if options.BufferedMessages == 0 {
		messages = make(chan Message)
	} else {
		messages = make(chan Message, options.BufferedMessages)
	}

	// Set up the context
	ctx, ctxCancel := context.WithCancel(parentCtx)

	// Initialize the connection object
	c := &conn{
		conn: rw,
		w:    w,
		p:    p,

		ctx:       ctx,
		ctxCancel: ctxCancel,

		readDone: done_signal.New(),
		closed:   done_signal.New(),
		done:     done_signal.New(),

		messages:    messages,
		responses:   make(chan Response),  // TODO: test if that's actually needed
		fastReplies: make(chan FastReply), // TODO: test if that's actually needed
	}

	// Start reading packets
	go c.handle()

	return c, nil
}

// Internals

func (c *conn) handle() {
	for {
		select {
		// When context has been terminated,
		// cancel all actions.
		case <-c.ctx.Done():
			c.markConnectionAsClosed()
			c.markReadAsDone()
			return

		default:
			// Parse next item
			v, err := c.p.Read()

			// Handle end of stream
			if err != nil {
				if err == io.EOF || strings.Contains(err.Error(), net.ErrClosed.Error()) {
					c.markReadAsDone()
					c.markConnectionAsClosed()
					return
				}

				// TODO: Expose errors
				panic(err)
			}

			switch item := v.(type) {
			case *message:
				c.messages <- item
			case Response:
				c.handleResponse(item)
				putOptional(c.responses, item)
			case FastReply:
				c.handleFastReply(item)
				putOptional(c.fastReplies, item)
			case GoAway:
				// TODO: Handle that?
			}
		}
	}
}

func (c *conn) handleResponse(r Response) {
	// TODO:
}

func (c *conn) handleFastReply(f FastReply) {
	// TODO:
}

func (c *conn) markReadAsDone() {
	if c.readDone.Close() && c.closed.Load() {
		c.markAsDone()
	}
}

func (c *conn) markConnectionAsClosed() {
	if c.closed.Close() && c.closed.Load() {
		c.markAsDone()
	}
}

func (c *conn) markAsDone() {
	if !c.done.Close() {
		return
	}

	// Cancel the context in case it's not closed yet
	c.ctxCancel()

	// Close all internal channels
	close(c.messages)
	close(c.fastReplies)
	close(c.responses)
}

// Implementation

func (c *conn) LocalAddr() net.Addr {
	if conn, ok := c.conn.(net.Conn); ok {
		return conn.LocalAddr()
	}
	return nil
}

func (c *conn) RemoteAddr() net.Addr {
	if conn, ok := c.conn.(net.Conn); ok {
		return conn.RemoteAddr()
	}
	return nil
}

func (c *conn) Request(p ProducerWithRequest) Request {
	return p.create(c.w)
}

func (c *conn) Pass(p Producer) error {
	return p.pass(c.w)
}

// TODO: Consider "force bool" argument (or ForceClose/GracefulClose methods), along with GOAWAY packet sent
func (c *conn) Close() error {
	// TODO: When force is false, probably end streams and wait for finish
	c.ctxCancel()

	// Close the connection TODO: think if it's needed, given that ctx is cancelled
	if cc, ok := c.conn.(net.Conn); ok {
		return cc.Close()
	}
	return nil
}

func (c *conn) ReadDone() <-chan struct{} {
	return c.readDone.Get()
}

func (c *conn) ConnectionClosed() <-chan struct{} {
	return c.closed.Get()
}

func (c *conn) Done() <-chan struct{} {
	return c.done.Get()
}

func (c *conn) Messages() <-chan Message {
	return c.messages
}

func (c *conn) Responses() <-chan Response {
	return c.responses
}

func (c *conn) FastReplies() <-chan FastReply {
	return c.fastReplies
}
