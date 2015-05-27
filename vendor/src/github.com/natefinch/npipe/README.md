npipe  [![Build status](https://ci.appveyor.com/api/projects/status/00vuepirsot29qwi)](https://ci.appveyor.com/project/natefinch/npipe) [![GoDoc](https://godoc.org/gopkg.in/natefinch/npipe.v2?status.svg)](https://godoc.org/gopkg.in/natefinch/npipe.v2)
=====
Package npipe provides a pure Go wrapper around Windows named pipes.

Windows named pipe documentation: http://msdn.microsoft.com/en-us/library/windows/desktop/aa365780

Note that the code lives at https://github.com/natefinch/npipe (v2 branch)
but should be imported as gopkg.in/natefinch/npipe.v2 (the package name is
still npipe).

npipe provides an interface based on stdlib's net package, with Dial, Listen,
and Accept functions, as well as associated implementations of net.Conn and
net.Listener.  It supports rpc over the connection.

### Notes
* Deadlines for reading/writing to the connection are only functional in Windows Vista/Server 2008 and above, due to limitations with the Windows API.

* The pipes support byte mode only (no support for message mode)

### Examples
The Dial function connects a client to a named pipe:


	conn, err := npipe.Dial(`\\.\pipe\mypipename`)
	if err != nil {
		<handle error>
	}
	fmt.Fprintf(conn, "Hi server!\n")
	msg, err := bufio.NewReader(conn).ReadString('\n')
	...

The Listen function creates servers:


	ln, err := npipe.Listen(`\\.\pipe\mypipename`)
	if err != nil {
		// handle error
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
			continue
		}
		go handleConnection(conn)
	}





## Variables
``` go
var ErrClosed = PipeError{"Pipe has been closed.", false}
```
ErrClosed is the error returned by PipeListener.Accept when Close is called
on the PipeListener.



## type PipeAddr
``` go
type PipeAddr string
```
PipeAddr represents the address of a named pipe.











### func (PipeAddr) Network
``` go
func (a PipeAddr) Network() string
```
Network returns the address's network name, "pipe".



### func (PipeAddr) String
``` go
func (a PipeAddr) String() string
```
String returns the address of the pipe



## type PipeConn
``` go
type PipeConn struct {
    // contains filtered or unexported fields
}
```
PipeConn is the implementation of the net.Conn interface for named pipe connections.









### func Dial
``` go
func Dial(address string) (*PipeConn, error)
```
Dial connects to a named pipe with the given address. If the specified pipe is not available,
it will wait indefinitely for the pipe to become available.

The address must be of the form \\.\\pipe\<name> for local pipes and \\<computer>\pipe\<name>
for remote pipes.

Dial will return a PipeError if you pass in a badly formatted pipe name.

Examples:


	// local pipe
	conn, err := Dial(`\\.\pipe\mypipename`)
	
	// remote pipe
	conn, err := Dial(`\\othercomp\pipe\mypipename`)


### func DialTimeout
``` go
func DialTimeout(address string, timeout time.Duration) (*PipeConn, error)
```
DialTimeout acts like Dial, but will time out after the duration of timeout




### func (\*PipeConn) Close
``` go
func (c *PipeConn) Close() error
```
Close closes the connection.



### func (\*PipeConn) LocalAddr
``` go
func (c *PipeConn) LocalAddr() net.Addr
```
LocalAddr returns the local network address.



### func (\*PipeConn) Read
``` go
func (c *PipeConn) Read(b []byte) (int, error)
```
Read implements the net.Conn Read method.



### func (\*PipeConn) RemoteAddr
``` go
func (c *PipeConn) RemoteAddr() net.Addr
```
RemoteAddr returns the remote network address.



### func (\*PipeConn) SetDeadline
``` go
func (c *PipeConn) SetDeadline(t time.Time) error
```
SetDeadline implements the net.Conn SetDeadline method.
Note that timeouts are only supported on Windows Vista/Server 2008 and above



### func (\*PipeConn) SetReadDeadline
``` go
func (c *PipeConn) SetReadDeadline(t time.Time) error
```
SetReadDeadline implements the net.Conn SetReadDeadline method.
Note that timeouts are only supported on Windows Vista/Server 2008 and above



### func (\*PipeConn) SetWriteDeadline
``` go
func (c *PipeConn) SetWriteDeadline(t time.Time) error
```
SetWriteDeadline implements the net.Conn SetWriteDeadline method.
Note that timeouts are only supported on Windows Vista/Server 2008 and above



### func (\*PipeConn) Write
``` go
func (c *PipeConn) Write(b []byte) (int, error)
```
Write implements the net.Conn Write method.



## type PipeError
``` go
type PipeError struct {
    // contains filtered or unexported fields
}
```
PipeError is an error related to a call to a pipe











### func (PipeError) Error
``` go
func (e PipeError) Error() string
```
Error implements the error interface



### func (PipeError) Temporary
``` go
func (e PipeError) Temporary() bool
```
Temporary implements net.AddrError.Temporary()



### func (PipeError) Timeout
``` go
func (e PipeError) Timeout() bool
```
Timeout implements net.AddrError.Timeout()



## type PipeListener
``` go
type PipeListener struct {
    // contains filtered or unexported fields
}
```
PipeListener is a named pipe listener. Clients should typically
use variables of type net.Listener instead of assuming named pipe.









### func Listen
``` go
func Listen(address string) (*PipeListener, error)
```
Listen returns a new PipeListener that will listen on a pipe with the given
address. The address must be of the form \\.\pipe\<name>

Listen will return a PipeError for an incorrectly formatted pipe name.




### func (\*PipeListener) Accept
``` go
func (l *PipeListener) Accept() (net.Conn, error)
```
Accept implements the Accept method in the net.Listener interface; it
waits for the next call and returns a generic net.Conn.



### func (\*PipeListener) AcceptPipe
``` go
func (l *PipeListener) AcceptPipe() (*PipeConn, error)
```
AcceptPipe accepts the next incoming call and returns the new connection.



### func (\*PipeListener) Addr
``` go
func (l *PipeListener) Addr() net.Addr
```
Addr returns the listener's network address, a PipeAddr.



### func (\*PipeListener) Close
``` go
func (l *PipeListener) Close() error
```
Close stops listening on the address.
Already Accepted connections are not closed.
