package npipe

//sys createNamedPipe(name *uint16, openMode uint32, pipeMode uint32, maxInstances uint32, outBufSize uint32, inBufSize uint32, defaultTimeout uint32, sa *syscall.SecurityAttributes) (handle syscall.Handle, err error)  [failretval==syscall.InvalidHandle] = CreateNamedPipeW
//sys connectNamedPipe(handle syscall.Handle, overlapped *syscall.Overlapped) (err error) = ConnectNamedPipe
//sys disconnectNamedPipe(handle syscall.Handle) (err error) = DisconnectNamedPipe
//sys waitNamedPipe(name *uint16, timeout uint32) (err error) = WaitNamedPipeW
//sys createEvent(sa *syscall.SecurityAttributes, manualReset bool, initialState bool, name *uint16) (handle syscall.Handle, err error) [failretval==syscall.InvalidHandle] = CreateEventW
//sys getOverlappedResult(handle syscall.Handle, overlapped *syscall.Overlapped, transferred *uint32, wait bool) (err error) = GetOverlappedResult
//sys cancelIoEx(handle syscall.Handle, overlapped *syscall.Overlapped) (err error) = CancelIoEx

import (
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

const (
	// openMode
	pipe_access_duplex   = 0x3
	pipe_access_inbound  = 0x1
	pipe_access_outbound = 0x2

	// openMode write flags
	file_flag_first_pipe_instance = 0x00080000
	file_flag_write_through       = 0x80000000
	file_flag_overlapped          = 0x40000000

	// openMode ACL flags
	write_dac              = 0x00040000
	write_owner            = 0x00080000
	access_system_security = 0x01000000

	// pipeMode
	pipe_type_byte    = 0x0
	pipe_type_message = 0x4

	// pipeMode read mode flags
	pipe_readmode_byte    = 0x0
	pipe_readmode_message = 0x2

	// pipeMode wait mode flags
	pipe_wait   = 0x0
	pipe_nowait = 0x1

	// pipeMode remote-client mode flags
	pipe_accept_remote_clients = 0x0
	pipe_reject_remote_clients = 0x8

	pipe_unlimited_instances = 255

	nmpwait_wait_forever = 0xFFFFFFFF

	// the two not-an-errors below occur if a client connects to the pipe between
	// the server's CreateNamedPipe and ConnectNamedPipe calls.
	error_no_data        syscall.Errno = 0xE8
	error_pipe_connected syscall.Errno = 0x217
	error_pipe_busy      syscall.Errno = 0xE7
	error_sem_timeout    syscall.Errno = 0x79

	error_bad_pathname syscall.Errno = 0xA1
	error_invalid_name syscall.Errno = 0x7B

	error_io_incomplete syscall.Errno = 0x3e4
)

var _ net.Conn = (*PipeConn)(nil)
var _ net.Listener = (*PipeListener)(nil)

// ErrClosed is the error returned by PipeListener.Accept when Close is called
// on the PipeListener.
var ErrClosed = PipeError{"Pipe has been closed.", false}

// PipeError is an error related to a call to a pipe
type PipeError struct {
	msg     string
	timeout bool
}

// Error implements the error interface
func (e PipeError) Error() string {
	return e.msg
}

// Timeout implements net.AddrError.Timeout()
func (e PipeError) Timeout() bool {
	return e.timeout
}

// Temporary implements net.AddrError.Temporary()
func (e PipeError) Temporary() bool {
	return false
}

// Dial connects to a named pipe with the given address. If the specified pipe is not available,
// it will wait indefinitely for the pipe to become available.
//
// The address must be of the form \\.\\pipe\<name> for local pipes and \\<computer>\pipe\<name>
// for remote pipes.
//
// Dial will return a PipeError if you pass in a badly formatted pipe name.
//
// Examples:
//   // local pipe
//   conn, err := Dial(`\\.\pipe\mypipename`)
//
//   // remote pipe
//   conn, err := Dial(`\\othercomp\pipe\mypipename`)
func Dial(address string) (*PipeConn, error) {
	for {
		conn, err := dial(address, nmpwait_wait_forever)
		if err == nil {
			return conn, nil
		}
		if isPipeNotReady(err) {
			<-time.After(100 * time.Millisecond)
			continue
		}
		return nil, err
	}
}

// DialTimeout acts like Dial, but will time out after the duration of timeout
func DialTimeout(address string, timeout time.Duration) (*PipeConn, error) {
	deadline := time.Now().Add(timeout)

	now := time.Now()
	for now.Before(deadline) {
		millis := uint32(deadline.Sub(now) / time.Millisecond)
		conn, err := dial(address, millis)
		if err == nil {
			return conn, nil
		}
		if err == error_sem_timeout {
			// This is WaitNamedPipe's timeout error, so we know we're done
			return nil, PipeError{fmt.Sprintf(
				"Timed out waiting for pipe '%s' to come available", address), true}
		}
		if isPipeNotReady(err) {
			left := deadline.Sub(time.Now())
			retry := 100 * time.Millisecond
			if left > retry {
				<-time.After(retry)
			} else {
				<-time.After(left - time.Millisecond)
			}
			now = time.Now()
			continue
		}
		return nil, err
	}
	return nil, PipeError{fmt.Sprintf(
		"Timed out waiting for pipe '%s' to come available", address), true}
}

// isPipeNotReady checks the error to see if it indicates the pipe is not ready
func isPipeNotReady(err error) bool {
	// Pipe Busy means another client just grabbed the open pipe end,
	// and the server hasn't made a new one yet.
	// File Not Found means the server hasn't created the pipe yet.
	// Neither is a fatal error.

	return err == syscall.ERROR_FILE_NOT_FOUND || err == error_pipe_busy
}

// newOverlapped creates a structure used to track asynchronous
// I/O requests that have been issued.
func newOverlapped() (*syscall.Overlapped, error) {
	event, err := createEvent(nil, true, true, nil)
	if err != nil {
		return nil, err
	}
	return &syscall.Overlapped{HEvent: event}, nil
}

// waitForCompletion waits for an asynchronous I/O request referred to by overlapped to complete.
// This function returns the number of bytes transferred by the operation and an error code if
// applicable (nil otherwise).
func waitForCompletion(handle syscall.Handle, overlapped *syscall.Overlapped) (uint32, error) {
	_, err := syscall.WaitForSingleObject(overlapped.HEvent, syscall.INFINITE)
	if err != nil {
		return 0, err
	}
	var transferred uint32
	err = getOverlappedResult(handle, overlapped, &transferred, true)
	return transferred, err
}

// dial is a helper to initiate a connection to a named pipe that has been started by a server.
// The timeout is only enforced if the pipe server has already created the pipe, otherwise
// this function will return immediately.
func dial(address string, timeout uint32) (*PipeConn, error) {
	name, err := syscall.UTF16PtrFromString(string(address))
	if err != nil {
		return nil, err
	}
	// If at least one instance of the pipe has been created, this function
	// will wait timeout milliseconds for it to become available.
	// It will return immediately regardless of timeout, if no instances
	// of the named pipe have been created yet.
	// If this returns with no error, there is a pipe available.
	if err := waitNamedPipe(name, timeout); err != nil {
		if err == error_bad_pathname {
			// badly formatted pipe name
			return nil, badAddr(address)
		}
		return nil, err
	}
	pathp, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(pathp, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		uint32(syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE), nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}
	return &PipeConn{handle: handle, addr: PipeAddr(address)}, nil
}

// Listen returns a new PipeListener that will listen on a pipe with the given
// address. The address must be of the form \\.\pipe\<name>
//
// Listen will return a PipeError for an incorrectly formatted pipe name.
func Listen(address string) (*PipeListener, error) {
	handle, err := createPipe(address, true)
	if err == error_invalid_name {
		return nil, badAddr(address)
	}
	if err != nil {
		return nil, err
	}
	return &PipeListener{
		addr:   PipeAddr(address),
		handle: handle,
	}, nil
}

// PipeListener is a named pipe listener. Clients should typically
// use variables of type net.Listener instead of assuming named pipe.
type PipeListener struct {
	addr   PipeAddr
	handle syscall.Handle
	closed bool

	// acceptHandle contains the current handle waiting for
	// an incoming connection or nil.
	acceptHandle syscall.Handle
	// acceptOverlapped is set before waiting on a connection.
	// If not waiting, it is nil.
	acceptOverlapped *syscall.Overlapped
	// acceptMutex protects the handle and overlapped structure.
	acceptMutex sync.Mutex
}

// Accept implements the Accept method in the net.Listener interface; it
// waits for the next call and returns a generic net.Conn.
func (l *PipeListener) Accept() (net.Conn, error) {
	c, err := l.AcceptPipe()
	for err == error_no_data {
		// Ignore clients that connect and immediately disconnect.
		c, err = l.AcceptPipe()
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// AcceptPipe accepts the next incoming call and returns the new connection.
// It might return an error if a client connected and immediately cancelled
// the connection.
func (l *PipeListener) AcceptPipe() (*PipeConn, error) {
	if l == nil || l.addr == "" || l.closed {
		return nil, syscall.EINVAL
	}

	// the first time we call accept, the handle will have been created by the Listen
	// call. This is to prevent race conditions where the client thinks the server
	// isn't listening because it hasn't actually called create yet. After the first time, we'll
	// have to create a new handle each time
	handle := l.handle
	if handle == 0 {
		var err error
		handle, err = createPipe(string(l.addr), false)
		if err != nil {
			return nil, err
		}
	} else {
		l.handle = 0
	}

	overlapped, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(overlapped.HEvent)
	if err := connectNamedPipe(handle, overlapped); err != nil && err != error_pipe_connected {
		if err == error_io_incomplete || err == syscall.ERROR_IO_PENDING {
			l.acceptMutex.Lock()
			l.acceptOverlapped = overlapped
			l.acceptHandle = handle
			l.acceptMutex.Unlock()
			defer func() {
				l.acceptMutex.Lock()
				l.acceptOverlapped = nil
				l.acceptHandle = 0
				l.acceptMutex.Unlock()
			}()

			_, err = waitForCompletion(handle, overlapped)
		}
		if err == syscall.ERROR_OPERATION_ABORTED {
			// Return error compatible to net.Listener.Accept() in case the
			// listener was closed.
			return nil, ErrClosed
		}
		if err != nil {
			return nil, err
		}
	}
	return &PipeConn{handle: handle, addr: l.addr}, nil
}

// Close stops listening on the address.
// Already Accepted connections are not closed.
func (l *PipeListener) Close() error {
	if l.closed {
		return nil
	}
	l.closed = true
	if l.handle != 0 {
		err := disconnectNamedPipe(l.handle)
		if err != nil {
			return err
		}
		err = syscall.CloseHandle(l.handle)
		if err != nil {
			return err
		}
		l.handle = 0
	}
	l.acceptMutex.Lock()
	defer l.acceptMutex.Unlock()
	if l.acceptOverlapped != nil && l.acceptHandle != 0 {
		// Cancel the pending IO. This call does not block, so it is safe
		// to hold onto the mutex above.
		if err := cancelIoEx(l.acceptHandle, l.acceptOverlapped); err != nil {
			return err
		}
		err := syscall.CloseHandle(l.acceptOverlapped.HEvent)
		if err != nil {
			return err
		}
		l.acceptOverlapped.HEvent = 0
		err = syscall.CloseHandle(l.acceptHandle)
		if err != nil {
			return err
		}
		l.acceptHandle = 0
	}
	return nil
}

// Addr returns the listener's network address, a PipeAddr.
func (l *PipeListener) Addr() net.Addr { return l.addr }

// PipeConn is the implementation of the net.Conn interface for named pipe connections.
type PipeConn struct {
	handle syscall.Handle
	addr   PipeAddr

	// these aren't actually used yet
	readDeadline  *time.Time
	writeDeadline *time.Time
}

type iodata struct {
	n   uint32
	err error
}

// completeRequest looks at iodata to see if a request is pending. If so, it waits for it to either complete or to
// abort due to hitting the specified deadline. Deadline may be set to nil to wait forever. If no request is pending,
// the content of iodata is returned.
func (c *PipeConn) completeRequest(data iodata, deadline *time.Time, overlapped *syscall.Overlapped) (int, error) {
	if data.err == error_io_incomplete || data.err == syscall.ERROR_IO_PENDING {
		var timer <-chan time.Time
		if deadline != nil {
			if timeDiff := deadline.Sub(time.Now()); timeDiff > 0 {
				timer = time.After(timeDiff)
			}
		}
		done := make(chan iodata)
		go func() {
			n, err := waitForCompletion(c.handle, overlapped)
			done <- iodata{n, err}
		}()
		select {
		case data = <-done:
		case <-timer:
			syscall.CancelIoEx(c.handle, overlapped)
			data = iodata{0, timeout(c.addr.String())}
		}
	}
	// Windows will produce ERROR_BROKEN_PIPE upon closing
	// a handle on the other end of a connection. Go RPC
	// expects an io.EOF error in this case.
	if data.err == syscall.ERROR_BROKEN_PIPE {
		data.err = io.EOF
	}
	return int(data.n), data.err
}

// Read implements the net.Conn Read method.
func (c *PipeConn) Read(b []byte) (int, error) {
	// Use ReadFile() rather than Read() because the latter
	// contains a workaround that eats ERROR_BROKEN_PIPE.
	overlapped, err := newOverlapped()
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(overlapped.HEvent)
	var n uint32
	err = syscall.ReadFile(c.handle, b, &n, overlapped)
	return c.completeRequest(iodata{n, err}, c.readDeadline, overlapped)
}

// Write implements the net.Conn Write method.
func (c *PipeConn) Write(b []byte) (int, error) {
	overlapped, err := newOverlapped()
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(overlapped.HEvent)
	var n uint32
	err = syscall.WriteFile(c.handle, b, &n, overlapped)
	return c.completeRequest(iodata{n, err}, c.writeDeadline, overlapped)
}

// Close closes the connection.
func (c *PipeConn) Close() error {
	return syscall.CloseHandle(c.handle)
}

// LocalAddr returns the local network address.
func (c *PipeConn) LocalAddr() net.Addr {
	return c.addr
}

// RemoteAddr returns the remote network address.
func (c *PipeConn) RemoteAddr() net.Addr {
	// not sure what to do here, we don't have remote addr....
	return c.addr
}

// SetDeadline implements the net.Conn SetDeadline method.
// Note that timeouts are only supported on Windows Vista/Server 2008 and above
func (c *PipeConn) SetDeadline(t time.Time) error {
	c.SetReadDeadline(t)
	c.SetWriteDeadline(t)
	return nil
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
// Note that timeouts are only supported on Windows Vista/Server 2008 and above
func (c *PipeConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = &t
	return nil
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
// Note that timeouts are only supported on Windows Vista/Server 2008 and above
func (c *PipeConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = &t
	return nil
}

// PipeAddr represents the address of a named pipe.
type PipeAddr string

// Network returns the address's network name, "pipe".
func (a PipeAddr) Network() string { return "pipe" }

// String returns the address of the pipe
func (a PipeAddr) String() string {
	return string(a)
}

// createPipe is a helper function to make sure we always create pipes
// with the same arguments, since subsequent calls to create pipe need
// to use the same arguments as the first one. If first is set, fail
// if the pipe already exists.
func createPipe(address string, first bool) (syscall.Handle, error) {
	n, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return 0, err
	}
	mode := uint32(pipe_access_duplex | syscall.FILE_FLAG_OVERLAPPED)
	if first {
		mode |= file_flag_first_pipe_instance
	}
	return createNamedPipe(n,
		mode,
		pipe_type_byte,
		pipe_unlimited_instances,
		512, 512, 0, nil)
}

func badAddr(addr string) PipeError {
	return PipeError{fmt.Sprintf("Invalid pipe address '%s'.", addr), false}
}
func timeout(addr string) PipeError {
	return PipeError{fmt.Sprintf("Pipe IO timed out waiting for '%s'", addr), true}
}
