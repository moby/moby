/*
 *
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package transport defines and implements message oriented communication
// channel to complete various transactions (e.g., an RPC).  It is meant for
// grpc-internal usage and is not intended to be imported directly by users.
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"
)

const logLevel = 2

type bufferPool struct {
	pool sync.Pool
}

func newBufferPool() *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

func (p *bufferPool) get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

func (p *bufferPool) put(b *bytes.Buffer) {
	p.pool.Put(b)
}

// recvMsg represents the received msg from the transport. All transport
// protocol specific info has been removed.
type recvMsg struct {
	buffer *bytes.Buffer
	// nil: received some data
	// io.EOF: stream is completed. data is nil.
	// other non-nil error: transport failure. data is nil.
	err error
}

// recvBuffer is an unbounded channel of recvMsg structs.
//
// Note: recvBuffer differs from buffer.Unbounded only in the fact that it
// holds a channel of recvMsg structs instead of objects implementing "item"
// interface. recvBuffer is written to much more often and using strict recvMsg
// structs helps avoid allocation in "recvBuffer.put"
type recvBuffer struct {
	c       chan recvMsg
	mu      sync.Mutex
	backlog []recvMsg
	err     error
}

func newRecvBuffer() *recvBuffer {
	b := &recvBuffer{
		c: make(chan recvMsg, 1),
	}
	return b
}

func (b *recvBuffer) put(r recvMsg) {
	b.mu.Lock()
	if b.err != nil {
		b.mu.Unlock()
		// An error had occurred earlier, don't accept more
		// data or errors.
		return
	}
	b.err = r.err
	if len(b.backlog) == 0 {
		select {
		case b.c <- r:
			b.mu.Unlock()
			return
		default:
		}
	}
	b.backlog = append(b.backlog, r)
	b.mu.Unlock()
}

func (b *recvBuffer) load() {
	b.mu.Lock()
	if len(b.backlog) > 0 {
		select {
		case b.c <- b.backlog[0]:
			b.backlog[0] = recvMsg{}
			b.backlog = b.backlog[1:]
		default:
		}
	}
	b.mu.Unlock()
}

// get returns the channel that receives a recvMsg in the buffer.
//
// Upon receipt of a recvMsg, the caller should call load to send another
// recvMsg onto the channel if there is any.
func (b *recvBuffer) get() <-chan recvMsg {
	return b.c
}

// recvBufferReader implements io.Reader interface to read the data from
// recvBuffer.
type recvBufferReader struct {
	closeStream func(error) // Closes the client transport stream with the given error and nil trailer metadata.
	ctx         context.Context
	ctxDone     <-chan struct{} // cache of ctx.Done() (for performance).
	recv        *recvBuffer
	last        *bytes.Buffer // Stores the remaining data in the previous calls.
	err         error
	freeBuffer  func(*bytes.Buffer)
}

// Read reads the next len(p) bytes from last. If last is drained, it tries to
// read additional data from recv. It blocks if there no additional data available
// in recv. If Read returns any non-nil error, it will continue to return that error.
func (r *recvBufferReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.last != nil {
		// Read remaining data left in last call.
		copied, _ := r.last.Read(p)
		if r.last.Len() == 0 {
			r.freeBuffer(r.last)
			r.last = nil
		}
		return copied, nil
	}
	if r.closeStream != nil {
		n, r.err = r.readClient(p)
	} else {
		n, r.err = r.read(p)
	}
	return n, r.err
}

func (r *recvBufferReader) read(p []byte) (n int, err error) {
	select {
	case <-r.ctxDone:
		return 0, ContextErr(r.ctx.Err())
	case m := <-r.recv.get():
		return r.readAdditional(m, p)
	}
}

func (r *recvBufferReader) readClient(p []byte) (n int, err error) {
	// If the context is canceled, then closes the stream with nil metadata.
	// closeStream writes its error parameter to r.recv as a recvMsg.
	// r.readAdditional acts on that message and returns the necessary error.
	select {
	case <-r.ctxDone:
		// Note that this adds the ctx error to the end of recv buffer, and
		// reads from the head. This will delay the error until recv buffer is
		// empty, thus will delay ctx cancellation in Recv().
		//
		// It's done this way to fix a race between ctx cancel and trailer. The
		// race was, stream.Recv() may return ctx error if ctxDone wins the
		// race, but stream.Trailer() may return a non-nil md because the stream
		// was not marked as done when trailer is received. This closeStream
		// call will mark stream as done, thus fix the race.
		//
		// TODO: delaying ctx error seems like a unnecessary side effect. What
		// we really want is to mark the stream as done, and return ctx error
		// faster.
		r.closeStream(ContextErr(r.ctx.Err()))
		m := <-r.recv.get()
		return r.readAdditional(m, p)
	case m := <-r.recv.get():
		return r.readAdditional(m, p)
	}
}

func (r *recvBufferReader) readAdditional(m recvMsg, p []byte) (n int, err error) {
	r.recv.load()
	if m.err != nil {
		return 0, m.err
	}
	copied, _ := m.buffer.Read(p)
	if m.buffer.Len() == 0 {
		r.freeBuffer(m.buffer)
		r.last = nil
	} else {
		r.last = m.buffer
	}
	return copied, nil
}

type streamState uint32

const (
	streamActive    streamState = iota
	streamWriteDone             // EndStream sent
	streamReadDone              // EndStream received
	streamDone                  // the entire stream is finished.
)

// Stream represents an RPC in the transport layer.
type Stream struct {
	id           uint32
	st           ServerTransport    // nil for client side Stream
	ct           *http2Client       // nil for server side Stream
	ctx          context.Context    // the associated context of the stream
	cancel       context.CancelFunc // always nil for client side Stream
	done         chan struct{}      // closed at the end of stream to unblock writers. On the client side.
	doneFunc     func()             // invoked at the end of stream on client side.
	ctxDone      <-chan struct{}    // same as done chan but for server side. Cache of ctx.Done() (for performance)
	method       string             // the associated RPC method of the stream
	recvCompress string
	sendCompress string
	buf          *recvBuffer
	trReader     io.Reader
	fc           *inFlow
	wq           *writeQuota

	// Callback to state application's intentions to read data. This
	// is used to adjust flow control, if needed.
	requestRead func(int)

	headerChan       chan struct{} // closed to indicate the end of header metadata.
	headerChanClosed uint32        // set when headerChan is closed. Used to avoid closing headerChan multiple times.
	// headerValid indicates whether a valid header was received.  Only
	// meaningful after headerChan is closed (always call waitOnHeader() before
	// reading its value).  Not valid on server side.
	headerValid bool

	// hdrMu protects header and trailer metadata on the server-side.
	hdrMu sync.Mutex
	// On client side, header keeps the received header metadata.
	//
	// On server side, header keeps the header set by SetHeader(). The complete
	// header will merged into this after t.WriteHeader() is called.
	header  metadata.MD
	trailer metadata.MD // the key-value map of trailer metadata.

	noHeaders bool // set if the client never received headers (set only after the stream is done).

	// On the server-side, headerSent is atomically set to 1 when the headers are sent out.
	headerSent uint32

	state streamState

	// On client-side it is the status error received from the server.
	// On server-side it is unused.
	status *status.Status

	bytesReceived uint32 // indicates whether any bytes have been received on this stream
	unprocessed   uint32 // set if the server sends a refused stream or GOAWAY including this stream

	// contentSubtype is the content-subtype for requests.
	// this must be lowercase or the behavior is undefined.
	contentSubtype string
}

// isHeaderSent is only valid on the server-side.
func (s *Stream) isHeaderSent() bool {
	return atomic.LoadUint32(&s.headerSent) == 1
}

// updateHeaderSent updates headerSent and returns true
// if it was alreay set. It is valid only on server-side.
func (s *Stream) updateHeaderSent() bool {
	return atomic.SwapUint32(&s.headerSent, 1) == 1
}

func (s *Stream) swapState(st streamState) streamState {
	return streamState(atomic.SwapUint32((*uint32)(&s.state), uint32(st)))
}

func (s *Stream) compareAndSwapState(oldState, newState streamState) bool {
	return atomic.CompareAndSwapUint32((*uint32)(&s.state), uint32(oldState), uint32(newState))
}

func (s *Stream) getState() streamState {
	return streamState(atomic.LoadUint32((*uint32)(&s.state)))
}

func (s *Stream) waitOnHeader() {
	if s.headerChan == nil {
		// On the server headerChan is always nil since a stream originates
		// only after having received headers.
		return
	}
	select {
	case <-s.ctx.Done():
		// Close the stream to prevent headers/trailers from changing after
		// this function returns.
		s.ct.CloseStream(s, ContextErr(s.ctx.Err()))
		// headerChan could possibly not be closed yet if closeStream raced
		// with operateHeaders; wait until it is closed explicitly here.
		<-s.headerChan
	case <-s.headerChan:
	}
}

// RecvCompress returns the compression algorithm applied to the inbound
// message. It is empty string if there is no compression applied.
func (s *Stream) RecvCompress() string {
	s.waitOnHeader()
	return s.recvCompress
}

// SetSendCompress sets the compression algorithm to the stream.
func (s *Stream) SetSendCompress(str string) {
	s.sendCompress = str
}

// Done returns a channel which is closed when it receives the final status
// from the server.
func (s *Stream) Done() <-chan struct{} {
	return s.done
}

// Header returns the header metadata of the stream.
//
// On client side, it acquires the key-value pairs of header metadata once it is
// available. It blocks until i) the metadata is ready or ii) there is no header
// metadata or iii) the stream is canceled/expired.
//
// On server side, it returns the out header after t.WriteHeader is called.  It
// does not block and must not be called until after WriteHeader.
func (s *Stream) Header() (metadata.MD, error) {
	if s.headerChan == nil {
		// On server side, return the header in stream. It will be the out
		// header after t.WriteHeader is called.
		return s.header.Copy(), nil
	}
	s.waitOnHeader()
	if !s.headerValid {
		return nil, s.status.Err()
	}
	return s.header.Copy(), nil
}

// TrailersOnly blocks until a header or trailers-only frame is received and
// then returns true if the stream was trailers-only.  If the stream ends
// before headers are received, returns true, nil.  Client-side only.
func (s *Stream) TrailersOnly() bool {
	s.waitOnHeader()
	return s.noHeaders
}

// Trailer returns the cached trailer metedata. Note that if it is not called
// after the entire stream is done, it could return an empty MD. Client
// side only.
// It can be safely read only after stream has ended that is either read
// or write have returned io.EOF.
func (s *Stream) Trailer() metadata.MD {
	c := s.trailer.Copy()
	return c
}

// ContentSubtype returns the content-subtype for a request. For example, a
// content-subtype of "proto" will result in a content-type of
// "application/grpc+proto". This will always be lowercase.  See
// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests for
// more details.
func (s *Stream) ContentSubtype() string {
	return s.contentSubtype
}

// Context returns the context of the stream.
func (s *Stream) Context() context.Context {
	return s.ctx
}

// Method returns the method for the stream.
func (s *Stream) Method() string {
	return s.method
}

// Status returns the status received from the server.
// Status can be read safely only after the stream has ended,
// that is, after Done() is closed.
func (s *Stream) Status() *status.Status {
	return s.status
}

// SetHeader sets the header metadata. This can be called multiple times.
// Server side only.
// This should not be called in parallel to other data writes.
func (s *Stream) SetHeader(md metadata.MD) error {
	if md.Len() == 0 {
		return nil
	}
	if s.isHeaderSent() || s.getState() == streamDone {
		return ErrIllegalHeaderWrite
	}
	s.hdrMu.Lock()
	s.header = metadata.Join(s.header, md)
	s.hdrMu.Unlock()
	return nil
}

// SendHeader sends the given header metadata. The given metadata is
// combined with any metadata set by previous calls to SetHeader and
// then written to the transport stream.
func (s *Stream) SendHeader(md metadata.MD) error {
	return s.st.WriteHeader(s, md)
}

// SetTrailer sets the trailer metadata which will be sent with the RPC status
// by the server. This can be called multiple times. Server side only.
// This should not be called parallel to other data writes.
func (s *Stream) SetTrailer(md metadata.MD) error {
	if md.Len() == 0 {
		return nil
	}
	if s.getState() == streamDone {
		return ErrIllegalHeaderWrite
	}
	s.hdrMu.Lock()
	s.trailer = metadata.Join(s.trailer, md)
	s.hdrMu.Unlock()
	return nil
}

func (s *Stream) write(m recvMsg) {
	s.buf.put(m)
}

// Read reads all p bytes from the wire for this stream.
func (s *Stream) Read(p []byte) (n int, err error) {
	// Don't request a read if there was an error earlier
	if er := s.trReader.(*transportReader).er; er != nil {
		return 0, er
	}
	s.requestRead(len(p))
	return io.ReadFull(s.trReader, p)
}

// tranportReader reads all the data available for this Stream from the transport and
// passes them into the decoder, which converts them into a gRPC message stream.
// The error is io.EOF when the stream is done or another non-nil error if
// the stream broke.
type transportReader struct {
	reader io.Reader
	// The handler to control the window update procedure for both this
	// particular stream and the associated transport.
	windowHandler func(int)
	er            error
}

func (t *transportReader) Read(p []byte) (n int, err error) {
	n, err = t.reader.Read(p)
	if err != nil {
		t.er = err
		return
	}
	t.windowHandler(n)
	return
}

// BytesReceived indicates whether any bytes have been received on this stream.
func (s *Stream) BytesReceived() bool {
	return atomic.LoadUint32(&s.bytesReceived) == 1
}

// Unprocessed indicates whether the server did not process this stream --
// i.e. it sent a refused stream or GOAWAY including this stream ID.
func (s *Stream) Unprocessed() bool {
	return atomic.LoadUint32(&s.unprocessed) == 1
}

// GoString is implemented by Stream so context.String() won't
// race when printing %#v.
func (s *Stream) GoString() string {
	return fmt.Sprintf("<stream: %p, %v>", s, s.method)
}

// state of transport
type transportState int

const (
	reachable transportState = iota
	closing
	draining
)

// ServerConfig consists of all the configurations to establish a server transport.
type ServerConfig struct {
	MaxStreams            uint32
	AuthInfo              credentials.AuthInfo
	InTapHandle           tap.ServerInHandle
	StatsHandler          stats.Handler
	KeepaliveParams       keepalive.ServerParameters
	KeepalivePolicy       keepalive.EnforcementPolicy
	InitialWindowSize     int32
	InitialConnWindowSize int32
	WriteBufferSize       int
	ReadBufferSize        int
	ChannelzParentID      int64
	MaxHeaderListSize     *uint32
	HeaderTableSize       *uint32
}

// ConnectOptions covers all relevant options for communicating with the server.
type ConnectOptions struct {
	// UserAgent is the application user agent.
	UserAgent string
	// Dialer specifies how to dial a network address.
	Dialer func(context.Context, string) (net.Conn, error)
	// FailOnNonTempDialError specifies if gRPC fails on non-temporary dial errors.
	FailOnNonTempDialError bool
	// PerRPCCredentials stores the PerRPCCredentials required to issue RPCs.
	PerRPCCredentials []credentials.PerRPCCredentials
	// TransportCredentials stores the Authenticator required to setup a client
	// connection. Only one of TransportCredentials and CredsBundle is non-nil.
	TransportCredentials credentials.TransportCredentials
	// CredsBundle is the credentials bundle to be used. Only one of
	// TransportCredentials and CredsBundle is non-nil.
	CredsBundle credentials.Bundle
	// KeepaliveParams stores the keepalive parameters.
	KeepaliveParams keepalive.ClientParameters
	// StatsHandler stores the handler for stats.
	StatsHandler stats.Handler
	// InitialWindowSize sets the initial window size for a stream.
	InitialWindowSize int32
	// InitialConnWindowSize sets the initial window size for a connection.
	InitialConnWindowSize int32
	// WriteBufferSize sets the size of write buffer which in turn determines how much data can be batched before it's written on the wire.
	WriteBufferSize int
	// ReadBufferSize sets the size of read buffer, which in turn determines how much data can be read at most for one read syscall.
	ReadBufferSize int
	// ChannelzParentID sets the addrConn id which initiate the creation of this client transport.
	ChannelzParentID int64
	// MaxHeaderListSize sets the max (uncompressed) size of header list that is prepared to be received.
	MaxHeaderListSize *uint32
	// UseProxy specifies if a proxy should be used.
	UseProxy bool
}

// NewClientTransport establishes the transport with the required ConnectOptions
// and returns it to the caller.
func NewClientTransport(connectCtx, ctx context.Context, addr resolver.Address, opts ConnectOptions, onPrefaceReceipt func(), onGoAway func(GoAwayReason), onClose func()) (ClientTransport, error) {
	return newHTTP2Client(connectCtx, ctx, addr, opts, onPrefaceReceipt, onGoAway, onClose)
}

// Options provides additional hints and information for message
// transmission.
type Options struct {
	// Last indicates whether this write is the last piece for
	// this stream.
	Last bool
}

// CallHdr carries the information of a particular RPC.
type CallHdr struct {
	// Host specifies the peer's host.
	Host string

	// Method specifies the operation to perform.
	Method string

	// SendCompress specifies the compression algorithm applied on
	// outbound message.
	SendCompress string

	// Creds specifies credentials.PerRPCCredentials for a call.
	Creds credentials.PerRPCCredentials

	// ContentSubtype specifies the content-subtype for a request. For example, a
	// content-subtype of "proto" will result in a content-type of
	// "application/grpc+proto". The value of ContentSubtype must be all
	// lowercase, otherwise the behavior is undefined. See
	// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
	// for more details.
	ContentSubtype string

	PreviousAttempts int // value of grpc-previous-rpc-attempts header to set

	DoneFunc func() // called when the stream is finished
}

// ClientTransport is the common interface for all gRPC client-side transport
// implementations.
type ClientTransport interface {
	// Close tears down this transport. Once it returns, the transport
	// should not be accessed any more. The caller must make sure this
	// is called only once.
	Close(err error)

	// GracefulClose starts to tear down the transport: the transport will stop
	// accepting new RPCs and NewStream will return error. Once all streams are
	// finished, the transport will close.
	//
	// It does not block.
	GracefulClose()

	// Write sends the data for the given stream. A nil stream indicates
	// the write is to be performed on the transport as a whole.
	Write(s *Stream, hdr []byte, data []byte, opts *Options) error

	// NewStream creates a Stream for an RPC.
	NewStream(ctx context.Context, callHdr *CallHdr) (*Stream, error)

	// CloseStream clears the footprint of a stream when the stream is
	// not needed any more. The err indicates the error incurred when
	// CloseStream is called. Must be called when a stream is finished
	// unless the associated transport is closing.
	CloseStream(stream *Stream, err error)

	// Error returns a channel that is closed when some I/O error
	// happens. Typically the caller should have a goroutine to monitor
	// this in order to take action (e.g., close the current transport
	// and create a new one) in error case. It should not return nil
	// once the transport is initiated.
	Error() <-chan struct{}

	// GoAway returns a channel that is closed when ClientTransport
	// receives the draining signal from the server (e.g., GOAWAY frame in
	// HTTP/2).
	GoAway() <-chan struct{}

	// GetGoAwayReason returns the reason why GoAway frame was received, along
	// with a human readable string with debug info.
	GetGoAwayReason() (GoAwayReason, string)

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr

	// IncrMsgSent increments the number of message sent through this transport.
	IncrMsgSent()

	// IncrMsgRecv increments the number of message received through this transport.
	IncrMsgRecv()
}

// ServerTransport is the common interface for all gRPC server-side transport
// implementations.
//
// Methods may be called concurrently from multiple goroutines, but
// Write methods for a given Stream will be called serially.
type ServerTransport interface {
	// HandleStreams receives incoming streams using the given handler.
	HandleStreams(func(*Stream), func(context.Context, string) context.Context)

	// WriteHeader sends the header metadata for the given stream.
	// WriteHeader may not be called on all streams.
	WriteHeader(s *Stream, md metadata.MD) error

	// Write sends the data for the given stream.
	// Write may not be called on all streams.
	Write(s *Stream, hdr []byte, data []byte, opts *Options) error

	// WriteStatus sends the status of a stream to the client.  WriteStatus is
	// the final call made on a stream and always occurs.
	WriteStatus(s *Stream, st *status.Status) error

	// Close tears down the transport. Once it is called, the transport
	// should not be accessed any more. All the pending streams and their
	// handlers will be terminated asynchronously.
	Close()

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr

	// Drain notifies the client this ServerTransport stops accepting new RPCs.
	Drain()

	// IncrMsgSent increments the number of message sent through this transport.
	IncrMsgSent()

	// IncrMsgRecv increments the number of message received through this transport.
	IncrMsgRecv()
}

// connectionErrorf creates an ConnectionError with the specified error description.
func connectionErrorf(temp bool, e error, format string, a ...interface{}) ConnectionError {
	return ConnectionError{
		Desc: fmt.Sprintf(format, a...),
		temp: temp,
		err:  e,
	}
}

// ConnectionError is an error that results in the termination of the
// entire connection and the retry of all the active streams.
type ConnectionError struct {
	Desc string
	temp bool
	err  error
}

func (e ConnectionError) Error() string {
	return fmt.Sprintf("connection error: desc = %q", e.Desc)
}

// Temporary indicates if this connection error is temporary or fatal.
func (e ConnectionError) Temporary() bool {
	return e.temp
}

// Origin returns the original error of this connection error.
func (e ConnectionError) Origin() error {
	// Never return nil error here.
	// If the original error is nil, return itself.
	if e.err == nil {
		return e
	}
	return e.err
}

var (
	// ErrConnClosing indicates that the transport is closing.
	ErrConnClosing = connectionErrorf(true, nil, "transport is closing")
	// errStreamDrain indicates that the stream is rejected because the
	// connection is draining. This could be caused by goaway or balancer
	// removing the address.
	errStreamDrain = status.Error(codes.Unavailable, "the connection is draining")
	// errStreamDone is returned from write at the client side to indiacte application
	// layer of an error.
	errStreamDone = errors.New("the stream is done")
	// StatusGoAway indicates that the server sent a GOAWAY that included this
	// stream's ID in unprocessed RPCs.
	statusGoAway = status.New(codes.Unavailable, "the stream is rejected because server is draining the connection")
)

// GoAwayReason contains the reason for the GoAway frame received.
type GoAwayReason uint8

const (
	// GoAwayInvalid indicates that no GoAway frame is received.
	GoAwayInvalid GoAwayReason = 0
	// GoAwayNoReason is the default value when GoAway frame is received.
	GoAwayNoReason GoAwayReason = 1
	// GoAwayTooManyPings indicates that a GoAway frame with
	// ErrCodeEnhanceYourCalm was received and that the debug data said
	// "too_many_pings".
	GoAwayTooManyPings GoAwayReason = 2
)

// channelzData is used to store channelz related data for http2Client and http2Server.
// These fields cannot be embedded in the original structs (e.g. http2Client), since to do atomic
// operation on int64 variable on 32-bit machine, user is responsible to enforce memory alignment.
// Here, by grouping those int64 fields inside a struct, we are enforcing the alignment.
type channelzData struct {
	kpCount int64
	// The number of streams that have started, including already finished ones.
	streamsStarted int64
	// Client side: The number of streams that have ended successfully by receiving
	// EoS bit set frame from server.
	// Server side: The number of streams that have ended successfully by sending
	// frame with EoS bit set.
	streamsSucceeded int64
	streamsFailed    int64
	// lastStreamCreatedTime stores the timestamp that the last stream gets created. It is of int64 type
	// instead of time.Time since it's more costly to atomically update time.Time variable than int64
	// variable. The same goes for lastMsgSentTime and lastMsgRecvTime.
	lastStreamCreatedTime int64
	msgSent               int64
	msgRecv               int64
	lastMsgSentTime       int64
	lastMsgRecvTime       int64
}

// ContextErr converts the error from context package into a status error.
func ContextErr(err error) error {
	switch err {
	case context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, err.Error())
	case context.Canceled:
		return status.Error(codes.Canceled, err.Error())
	}
	return status.Errorf(codes.Internal, "Unexpected error from context packet: %v", err)
}
