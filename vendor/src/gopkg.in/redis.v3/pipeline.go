package redis

import (
	"sync"
	"sync/atomic"
)

// Pipeline implements pipelining as described in
// http://redis.io/topics/pipelining. It's safe for concurrent use
// by multiple goroutines.
type Pipeline struct {
	commandable

	client *baseClient

	mu   sync.Mutex // protects cmds
	cmds []Cmder

	closed int32
}

func (c *Client) Pipeline() *Pipeline {
	pipe := &Pipeline{
		client: c.baseClient,
		cmds:   make([]Cmder, 0, 10),
	}
	pipe.commandable.process = pipe.process
	return pipe
}

func (c *Client) Pipelined(fn func(*Pipeline) error) ([]Cmder, error) {
	pipe := c.Pipeline()
	if err := fn(pipe); err != nil {
		return nil, err
	}
	cmds, err := pipe.Exec()
	_ = pipe.Close()
	return cmds, err
}

func (pipe *Pipeline) process(cmd Cmder) {
	pipe.mu.Lock()
	pipe.cmds = append(pipe.cmds, cmd)
	pipe.mu.Unlock()
}

// Close closes the pipeline, releasing any open resources.
func (pipe *Pipeline) Close() error {
	atomic.StoreInt32(&pipe.closed, 1)
	pipe.Discard()
	return nil
}

func (pipe *Pipeline) isClosed() bool {
	return atomic.LoadInt32(&pipe.closed) == 1
}

// Discard resets the pipeline and discards queued commands.
func (pipe *Pipeline) Discard() error {
	defer pipe.mu.Unlock()
	pipe.mu.Lock()
	if pipe.isClosed() {
		return errClosed
	}
	pipe.cmds = pipe.cmds[:0]
	return nil
}

// Exec executes all previously queued commands using one
// client-server roundtrip.
//
// Exec always returns list of commands and error of the first failed
// command if any.
func (pipe *Pipeline) Exec() (cmds []Cmder, retErr error) {
	if pipe.isClosed() {
		return nil, errClosed
	}

	defer pipe.mu.Unlock()
	pipe.mu.Lock()

	if len(pipe.cmds) == 0 {
		return pipe.cmds, nil
	}

	cmds = pipe.cmds
	pipe.cmds = make([]Cmder, 0, 10)

	failedCmds := cmds
	for i := 0; i <= pipe.client.opt.MaxRetries; i++ {
		cn, _, err := pipe.client.conn()
		if err != nil {
			setCmdsErr(failedCmds, err)
			return cmds, err
		}

		if i > 0 {
			resetCmds(failedCmds)
		}
		failedCmds, err = execCmds(cn, failedCmds)
		pipe.client.putConn(cn, err)
		if err != nil && retErr == nil {
			retErr = err
		}
		if len(failedCmds) == 0 {
			break
		}
	}

	return cmds, retErr
}

func execCmds(cn *conn, cmds []Cmder) ([]Cmder, error) {
	if err := cn.writeCmds(cmds...); err != nil {
		setCmdsErr(cmds, err)
		return cmds, err
	}

	var firstCmdErr error
	var failedCmds []Cmder
	for _, cmd := range cmds {
		err := cmd.readReply(cn)
		if err == nil {
			continue
		}
		if firstCmdErr == nil {
			firstCmdErr = err
		}
		if shouldRetry(err) {
			failedCmds = append(failedCmds, cmd)
		}
	}

	return failedCmds, firstCmdErr
}
