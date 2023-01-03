package sockety

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ServerOptions struct {
	HandleError      func(e error) // default: ignore errors TODO?
	Timeout          time.Duration // default: none TODO?
	Channels         uint16        // default: max
	WriteChannels    uint16        // default: max
	ReadBufferSize   uint32        // per connection, default: 4096
	BufferedMessages uint16        // per connection, default: 0
	//MaxConnections   uint32        // default: infinity, TODO: consider
}

type server struct {
	// Configuration
	errorHandler func(e error)
	connOptions  *ConnOptions

	// State
	sendingMessages atomic.Bool
	connCount       uint32
	connCh          chan Conn
	ctx             context.Context
	ctxCancel       context.CancelFunc
	listener        atomic.Pointer[net.Listener]
	listenerMu      sync.Mutex
}

var ErrServerClosed = errors.New("server closed")

type Server interface {
	Listen(network string, address string) error
	ListenContext(context context.Context, network string, address string) error
	Close() error

	Accept() (Conn, error)
	Next() <-chan Conn
	Done() <-chan struct{}
}

func NewServer(options *ServerOptions) Server {
	if options == nil {
		options = &ServerOptions{}
	}
	errorHandler := options.HandleError
	if errorHandler == nil {
		errorHandler = func(e error) {}
	}

	connCh := make(chan Conn)
	close(connCh)

	return &server{
		errorHandler: errorHandler,
		connOptions: &ConnOptions{
			Channels:         options.Channels,
			WriteChannels:    options.WriteChannels,
			BufferedMessages: options.BufferedMessages,
			ReadBufferSize:   options.ReadBufferSize,
			Timeout:          options.Timeout,
		},

		connCh: connCh,
	}
}

func (s *server) handleServerError(e error) {
	s.errorHandler(e)
}

func (s *server) handleConnectionError(c Conn, e error) {
	// TODO: Close automatically?
	// TODO: Consider what to do
	s.errorHandler(e)
}

func (s *server) Next() <-chan Conn {
	return s.connCh
}

func (s *server) Accept() (Conn, error) {
	select {
	case <-s.Done():
		return nil, ErrServerClosed
	case c := <-s.Next():
		return c, nil
	}
}

func (s *server) handleConnection(c Conn) {
	s.connCh <- c
}

func (s *server) handleClose(ctx context.Context) {
	<-ctx.Done()
	close(s.connCh)
}

func (s *server) acceptConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			listenerPointer := s.listener.Load()
			if listenerPointer == nil || *listenerPointer == nil {
				return
			}
			listener := *listenerPointer

			raw, err := listener.Accept()
			if err != nil {
				if !strings.Contains(err.Error(), net.ErrClosed.Error()) && s.listener.Load() != nil {
					s.handleServerError(err)
				}
				return
			}

			c, err := NewConn(ctx, raw, s.connOptions)
			if err != nil {
				s.handleServerError(err)
			} else {
				s.handleConnection(c)
			}
		}
	}
}

func (s *server) Listen(network string, address string) error {
	return s.ListenContext(context.Background(), network, address)
}

func (s *server) ListenContext(parentCtx context.Context, network string, address string) error {
	// Ensure the network type is supported by Sockety
	validateNetwork(network)

	s.listenerMu.Lock()
	if s.listener.Load() != nil {
		s.listenerMu.Unlock()
		return errors.New("server is already listening")
	}

	s.ctx, s.ctxCancel = context.WithCancel(parentCtx)

	s.connCh = make(chan Conn)

	var lc net.ListenConfig
	listener, err := lc.Listen(s.ctx, network, address)
	if err != nil {
		s.listenerMu.Unlock()
		return err
	}
	s.listener.Store(&listener)
	s.listenerMu.Unlock()
	go s.handleClose(s.ctx)
	go s.acceptConnections(s.ctx)
	return nil
}

func (s *server) Close() error {
	s.listenerMu.Lock()
	listener := *s.listener.Load()
	if listener == nil {
		s.listenerMu.Unlock()
		return errors.New("server is not listening")
	}
	err := listener.Close()
	s.listener.Store(nil)
	s.listenerMu.Unlock()
	return err
}

func (s *server) Done() <-chan struct{} {
	if s.listener.Load() == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return s.ctx.Done()
}
