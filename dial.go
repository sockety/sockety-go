package sockety

import (
	"context"
	"net"
)

func Dial(network string, address string, options *ConnOptions) (Conn, error) {
	return DialContext(context.Background(), network, address, options)
}

func DialContext(context context.Context, network string, address string, options *ConnOptions) (Conn, error) {
	// Ensure the network type is supported by Sockety
	validateNetwork(network)

	// Connect
	tcp, err := new(net.Dialer).DialContext(context, network, address)
	if err != nil {
		return nil, err
	}

	// Optimize
	if tcp, ok := tcp.(*net.TCPConn); ok {
		err = tcp.SetNoDelay(false)
		if err != nil {
			return nil, err
		}
	}

	// Wrap
	c, err := NewConn(context, tcp, options)
	if err != nil {
		return nil, err
	}
	return c, nil
}
