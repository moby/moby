package beam

import (
	"fmt"
	"io"
	"os"
	"sync"
)

type pipe struct {
	r   chan Message
	w   chan Message
	err error
	// We need a mutex to synchronize Send() and Close()
	// Send() doesn't need the mutex because it's legal to
	// receive from a closed channel, so we use a failed receive
	// as a thread-safe "close" message.
	lock sync.Mutex
}

func (p *pipe) Receive() (msg Message, err error) {
	if p.err != nil {
		err = p.err
		return
	}
	var ok bool
	msg, ok = <-p.r
	if !ok {
		p.err = io.EOF
	}
	err = p.err
	return
}

func (p *pipe) Send(msg Message) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.err != nil {
		return fmt.Errorf("send: pipe closed")
	}
	if msg.Stream != nil {
		local, remote := Pipe()
		go Splice(local, remote)
		go Splice(local, msg.Stream)
		msg.Stream = remote
	}
	p.w <- msg
	return nil
}

func (p *pipe) File() (*os.File, error) {
	return nil, fmt.Errorf("no file descriptor associated with stream")
}

func (p *pipe) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.err != nil {
		return fmt.Errorf("close: pipe already closed")
	}
	p.err = io.EOF
	close(p.w)
	return nil
}

func Pipe() (Stream, Stream) {
	red := make(chan Message)
	black := make(chan Message)
	return &pipe{r: red, w: black}, &pipe{r: black, w: red}
}
