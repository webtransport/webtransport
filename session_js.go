//go:build js

package webtransport

import (
	"context"
	"fmt"
	"net"
	"syscall/js"
)

// Session represents a WebTransport connection that can open and accept
// bidirectional streams.
type Session struct {
	transport js.Value
	incoming  js.Value
	addr      string
	ctx       context.Context
	cancel    context.CancelFunc
}

func newSession(transport js.Value, addr string) (*Session, error) {
	incoming := transport.Get("incomingBidirectionalStreams")
	if incoming.IsUndefined() || incoming.IsNull() {
		return nil, fmt.Errorf("incoming bidirectional streams are unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		transport: transport,
		incoming:  incoming.Call("getReader"),
		addr:      addr,
		ctx:       ctx,
		cancel:    cancel,
	}
	go func() {
		_, _ = awaitPromise(context.Background(), transport.Get("closed"))
		cancel()
	}()
	return s, nil
}

// Accept waits for and returns the next incoming bidirectional stream.
func (s *Session) Accept(ctx context.Context) (net.Conn, error) {
	result, err := awaitPromise(ctx, s.incoming.Call("read"))
	if err != nil {
		return nil, err
	}
	if result.Get("done").Bool() {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		return nil, context.Canceled
	}
	return newConn(result.Get("value"), s.addr), nil
}

// Open creates and returns a new outgoing bidirectional stream.
func (s *Session) Open(ctx context.Context) (net.Conn, error) {
	stream, err := awaitPromise(ctx, s.transport.Call("createBidirectionalStream"))
	if err != nil {
		return nil, err
	}
	if stream.IsUndefined() || stream.IsNull() {
		return nil, fmt.Errorf("stream is empty")
	}
	return newConn(stream, s.addr), nil
}

// Close closes the session and releases associated resources.
func (s *Session) Close() error {
	s.cancel()
	s.transport.Call("close")
	if !(s.incoming.IsUndefined() || s.incoming.IsNull()) {
		_, _ = awaitPromise(context.Background(), s.incoming.Call("cancel"))
	}
	return nil
}

// Context returns a context that is canceled when the underlying transport is
// closed.
func (s *Session) Context() context.Context {
	return s.ctx
}
