package redis

import (
	"errors"
	"fmt"
	"log"
)

var errDiscard = errors.New("redis: Discard can be used only inside Exec")

// Multi implements Redis transactions as described in
// http://redis.io/topics/transactions. It's NOT safe for concurrent use
// by multiple goroutines, because Exec resets list of watched keys.
// If you don't need WATCH it is better to use Pipeline.
//
// TODO(vmihailenco): rename to Tx and rework API
type Multi struct {
	commandable

	base *baseClient

	cmds   []Cmder
	closed bool
}

// Watch creates new transaction and marks the keys to be watched
// for conditional execution of a transaction.
func (c *Client) Watch(keys ...string) (*Multi, error) {
	tx := c.Multi()
	if err := tx.Watch(keys...).Err(); err != nil {
		tx.Close()
		return nil, err
	}
	return tx, nil
}

// Deprecated. Use Watch instead.
func (c *Client) Multi() *Multi {
	multi := &Multi{
		base: &baseClient{
			opt:      c.opt,
			connPool: newStickyConnPool(c.connPool, true),
		},
	}
	multi.commandable.process = multi.process
	return multi
}

func (c *Multi) putConn(cn *conn, err error) {
	if isBadConn(cn, err) {
		// Close current connection.
		c.base.connPool.(*stickyConnPool).Reset(err)
	} else {
		err := c.base.connPool.Put(cn)
		if err != nil {
			log.Printf("redis: putConn failed: %s", err)
		}
	}
}

func (c *Multi) process(cmd Cmder) {
	if c.cmds == nil {
		c.base.process(cmd)
	} else {
		c.cmds = append(c.cmds, cmd)
	}
}

// Close closes the client, releasing any open resources.
func (c *Multi) Close() error {
	c.closed = true
	if err := c.Unwatch().Err(); err != nil {
		log.Printf("redis: Unwatch failed: %s", err)
	}
	return c.base.Close()
}

// Watch marks the keys to be watched for conditional execution
// of a transaction.
func (c *Multi) Watch(keys ...string) *StatusCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "WATCH"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

// Unwatch flushes all the previously watched keys for a transaction.
func (c *Multi) Unwatch(keys ...string) *StatusCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "UNWATCH"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

// Discard discards queued commands.
func (c *Multi) Discard() error {
	if c.cmds == nil {
		return errDiscard
	}
	c.cmds = c.cmds[:1]
	return nil
}

// Exec executes all previously queued commands in a transaction
// and restores the connection state to normal.
//
// When using WATCH, EXEC will execute commands only if the watched keys
// were not modified, allowing for a check-and-set mechanism.
//
// Exec always returns list of commands. If transaction fails
// TxFailedErr is returned. Otherwise Exec returns error of the first
// failed command or nil.
func (c *Multi) Exec(f func() error) ([]Cmder, error) {
	if c.closed {
		return nil, errClosed
	}

	c.cmds = []Cmder{NewStatusCmd("MULTI")}
	if err := f(); err != nil {
		return nil, err
	}
	c.cmds = append(c.cmds, NewSliceCmd("EXEC"))

	cmds := c.cmds
	c.cmds = nil

	if len(cmds) == 2 {
		return []Cmder{}, nil
	}

	// Strip MULTI and EXEC commands.
	retCmds := cmds[1 : len(cmds)-1]

	cn, _, err := c.base.conn()
	if err != nil {
		setCmdsErr(retCmds, err)
		return retCmds, err
	}

	err = c.execCmds(cn, cmds)
	c.putConn(cn, err)
	return retCmds, err
}

func (c *Multi) execCmds(cn *conn, cmds []Cmder) error {
	err := cn.writeCmds(cmds...)
	if err != nil {
		setCmdsErr(cmds[1:len(cmds)-1], err)
		return err
	}

	statusCmd := NewStatusCmd()

	// Omit last command (EXEC).
	cmdsLen := len(cmds) - 1

	// Parse queued replies.
	for i := 0; i < cmdsLen; i++ {
		if err := statusCmd.readReply(cn); err != nil {
			setCmdsErr(cmds[1:len(cmds)-1], err)
			return err
		}
	}

	// Parse number of replies.
	line, err := readLine(cn)
	if err != nil {
		if err == Nil {
			err = TxFailedErr
		}
		setCmdsErr(cmds[1:len(cmds)-1], err)
		return err
	}
	if line[0] != '*' {
		err := fmt.Errorf("redis: expected '*', but got line %q", line)
		setCmdsErr(cmds[1:len(cmds)-1], err)
		return err
	}

	var firstCmdErr error

	// Parse replies.
	// Loop starts from 1 to omit MULTI cmd.
	for i := 1; i < cmdsLen; i++ {
		cmd := cmds[i]
		if err := cmd.readReply(cn); err != nil {
			if firstCmdErr == nil {
				firstCmdErr = err
			}
		}
	}

	return firstCmdErr
}
