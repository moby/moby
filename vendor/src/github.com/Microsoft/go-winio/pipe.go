package winio

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"
)

//sys connectNamedPipe(pipe syscall.Handle, o *syscall.Overlapped) (err error) = ConnectNamedPipe
//sys createNamedPipe(name string, flags uint32, pipeMode uint32, maxInstances uint32, outSize uint32, inSize uint32, defaultTimeout uint32, sa *securityAttributes) (handle syscall.Handle, err error)  [failretval==syscall.InvalidHandle] = CreateNamedPipeW
//sys createFile(name string, access uint32, mode uint32, sa *securityAttributes, createmode uint32, attrs uint32, templatefile syscall.Handle) (handle syscall.Handle, err error) [failretval==syscall.InvalidHandle] = CreateFileW
//sys waitNamedPipe(name string, timeout uint32) (err error) = WaitNamedPipeW

type securityAttributes struct {
	Length             uint32
	SecurityDescriptor *byte
	InheritHandle      uint32
}

const (
	cERROR_PIPE_BUSY      = syscall.Errno(231)
	cERROR_PIPE_CONNECTED = syscall.Errno(535)
	cERROR_SEM_TIMEOUT    = syscall.Errno(121)

	cPIPE_ACCESS_DUPLEX            = 0x3
	cFILE_FLAG_FIRST_PIPE_INSTANCE = 0x80000
	cSECURITY_SQOS_PRESENT         = 0x100000
	cSECURITY_ANONYMOUS            = 0

	cPIPE_REJECT_REMOTE_CLIENTS = 0x8

	cPIPE_UNLIMITED_INSTANCES = 255

	cNMPWAIT_USE_DEFAULT_WAIT = 0
	cNMPWAIT_NOWAIT           = 1
)

var (
	// This error should match net.errClosing since docker takes a dependency on its text
	ErrPipeListenerClosed = errors.New("use of closed network connection")
)

type win32Pipe struct {
	*win32File
	path string
}

type pipeAddress string

func (f *win32Pipe) LocalAddr() net.Addr {
	return pipeAddress(f.path)
}

func (f *win32Pipe) RemoteAddr() net.Addr {
	return pipeAddress(f.path)
}

func (f *win32Pipe) SetDeadline(t time.Time) error {
	f.SetReadDeadline(t)
	f.SetWriteDeadline(t)
	return nil
}

func (s pipeAddress) Network() string {
	return "pipe"
}

func (s pipeAddress) String() string {
	return string(s)
}

func makeWin32Pipe(h syscall.Handle, path string) (*win32Pipe, error) {
	f, err := makeWin32File(h)
	if err != nil {
		return nil, err
	}
	return &win32Pipe{f, path}, nil
}

// DialPipe connects to a named pipe by path, timing out if the connection
// takes longer than the specified duration. If timeout is nil, then the timeout
// is the default timeout established by the pipe server.
func DialPipe(path string, timeout *time.Duration) (net.Conn, error) {
	var absTimeout time.Time
	if timeout != nil {
		absTimeout = time.Now().Add(*timeout)
	}
	var err error
	var h syscall.Handle
	for {
		h, err = createFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED|cSECURITY_SQOS_PRESENT|cSECURITY_ANONYMOUS, 0)
		if err != cERROR_PIPE_BUSY {
			break
		}
		now := time.Now()
		var ms uint32
		if absTimeout.IsZero() {
			ms = cNMPWAIT_USE_DEFAULT_WAIT
		} else if now.After(absTimeout) {
			ms = cNMPWAIT_NOWAIT
		} else {
			ms = uint32(absTimeout.Sub(now).Nanoseconds() / 1000 / 1000)
		}
		err = waitNamedPipe(path, ms)
		if err != nil {
			if err == cERROR_SEM_TIMEOUT {
				return nil, ErrTimeout
			}
			break
		}
	}
	if err != nil {
		return nil, &os.PathError{"open", path, err}
	}
	p, err := makeWin32Pipe(h, path)
	if err != nil {
		syscall.Close(h)
		return nil, err
	}
	return p, nil
}

type acceptResponse struct {
	p   *win32Pipe
	err error
}

type win32PipeListener struct {
	firstHandle        syscall.Handle
	path               string
	securityDescriptor []byte
	acceptCh           chan (chan acceptResponse)
	closeCh            chan int
	doneCh             chan int
}

func makeServerPipeHandle(path string, securityDescriptor []byte, first bool) (syscall.Handle, error) {
	var flags uint32 = cPIPE_ACCESS_DUPLEX | syscall.FILE_FLAG_OVERLAPPED
	if first {
		flags |= cFILE_FLAG_FIRST_PIPE_INSTANCE
	}
	var sa securityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	if securityDescriptor != nil {
		sa.SecurityDescriptor = &securityDescriptor[0]
	}
	h, err := createNamedPipe(path, flags, cPIPE_REJECT_REMOTE_CLIENTS, cPIPE_UNLIMITED_INSTANCES, 4096, 4096, 0, &sa)
	if err != nil {
		return 0, &os.PathError{"open", path, err}
	}
	return h, nil
}

func (l *win32PipeListener) makeServerPipe() (*win32Pipe, error) {
	h, err := makeServerPipeHandle(l.path, l.securityDescriptor, false)
	if err != nil {
		return nil, err
	}
	p, err := makeWin32Pipe(h, l.path)
	if err != nil {
		syscall.Close(h)
		return nil, err
	}
	return p, nil
}

func (l *win32PipeListener) listenerRoutine() {
	closed := false
	for !closed {
		select {
		case <-l.closeCh:
			closed = true
		case responseCh := <-l.acceptCh:
			p, err := l.makeServerPipe()
			if err == nil {
				// Wait for the client to connect.
				ch := make(chan error)
				go func() {
					ch <- connectPipe(p)
				}()
				select {
				case err = <-ch:
					if err != nil {
						p.Close()
						p = nil
					}
				case <-l.closeCh:
					// Abort the connect request by closing the handle.
					p.Close()
					p = nil
					err = <-ch
					if err == nil || err == ErrFileClosed {
						err = ErrPipeListenerClosed
					}
					closed = true
				}
			}
			responseCh <- acceptResponse{p, err}
		}
	}
	syscall.Close(l.firstHandle)
	l.firstHandle = 0
	// Notify Close() and Accept() callers that the handle has been closed.
	close(l.doneCh)
}

func ListenPipe(path, sddl string) (net.Listener, error) {
	var (
		sd  []byte
		err error
	)
	if sddl != "" {
		sd, err = SddlToSecurityDescriptor(sddl)
		if err != nil {
			return nil, err
		}
	}
	h, err := makeServerPipeHandle(path, sd, true)
	if err != nil {
		return nil, err
	}
	// Immediately open and then close a client handle so that the named pipe is
	// created but not currently accepting connections.
	h2, err := createFile(path, 0, 0, nil, syscall.OPEN_EXISTING, cSECURITY_SQOS_PRESENT|cSECURITY_ANONYMOUS, 0)
	if err != nil {
		syscall.Close(h)
		return nil, err
	}
	syscall.Close(h2)
	l := &win32PipeListener{
		firstHandle:        h,
		path:               path,
		securityDescriptor: sd,
		acceptCh:           make(chan (chan acceptResponse)),
		closeCh:            make(chan int),
		doneCh:             make(chan int),
	}
	go l.listenerRoutine()
	return l, nil
}

func connectPipe(p *win32Pipe) error {
	c, err := p.prepareIo()
	if err != nil {
		return err
	}
	err = connectNamedPipe(p.handle, &c.o)
	_, err = p.asyncIo(c, time.Time{}, 0, err)
	if err != nil && err != cERROR_PIPE_CONNECTED {
		return err
	}
	return nil
}

func (l *win32PipeListener) Accept() (net.Conn, error) {
	ch := make(chan acceptResponse)
	select {
	case l.acceptCh <- ch:
		response := <-ch
		return response.p, response.err
	case <-l.doneCh:
		return nil, ErrPipeListenerClosed
	}
}

func (l *win32PipeListener) Close() error {
	select {
	case l.closeCh <- 1:
		<-l.doneCh
	case <-l.doneCh:
	}
	return nil
}

func (l *win32PipeListener) Addr() net.Addr {
	return pipeAddress(l.path)
}
