package beam

import (
	"fmt"
	"io"
	"os"
	"sync"
)

type pipe struct {
	r chan *pipeMsg
	w chan *pipeMsg
	err error
	// We need a mutex to synchronize Send() and Close()
	// Send() doesn't need the mutex because it's legal to
	// receive from a closed channel, so we use a failed receive
	// as a thread-safe "close" message.
	lock sync.Mutex
}

type pipeMsg struct {
	data []byte
	stream Stream
}


func (p *pipe) Receive() (data []byte, stream Stream, err error) {
	if p.err != nil {
		return nil, nil, p.err
	}
	msg, ok := <-p.r
	if !ok {
		p.err = io.EOF
	} else {
		data = msg.data
		stream = msg.stream
	}
	err = p.err
	return
}

func (p *pipe) Send(data []byte, stream Stream) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.err != nil {
		return fmt.Errorf("send: pipe closed")
	}
	msg := &pipeMsg{data: data}
	if stream != nil {
		local, remote := Pipe()
		go Splice(local, remote)
		go Splice(local, stream)
		msg.stream = remote
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
	red := make(chan *pipeMsg)
	black := make(chan *pipeMsg)
	return &pipe{r: red, w: black}, &pipe{r: black, w: red}
}
