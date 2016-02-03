package redis

import (
	"fmt"
	"log"
	"net"
	"time"
)

// Posts a message to the given channel.
func (c *Client) Publish(channel, message string) *IntCmd {
	req := NewIntCmd("PUBLISH", channel, message)
	c.Process(req)
	return req
}

// PubSub implements Pub/Sub commands as described in
// http://redis.io/topics/pubsub. It's NOT safe for concurrent use by
// multiple goroutines.
type PubSub struct {
	*baseClient

	channels []string
	patterns []string
}

// Deprecated. Use Subscribe/PSubscribe instead.
func (c *Client) PubSub() *PubSub {
	return &PubSub{
		baseClient: &baseClient{
			opt:      c.opt,
			connPool: newStickyConnPool(c.connPool, false),
		},
	}
}

// Subscribes the client to the specified channels.
func (c *Client) Subscribe(channels ...string) (*PubSub, error) {
	pubsub := c.PubSub()
	return pubsub, pubsub.Subscribe(channels...)
}

// Subscribes the client to the given patterns.
func (c *Client) PSubscribe(channels ...string) (*PubSub, error) {
	pubsub := c.PubSub()
	return pubsub, pubsub.PSubscribe(channels...)
}

func (c *PubSub) subscribe(cmd string, channels ...string) error {
	cn, _, err := c.conn()
	if err != nil {
		return err
	}

	args := make([]interface{}, 1+len(channels))
	args[0] = cmd
	for i, channel := range channels {
		args[1+i] = channel
	}
	req := NewSliceCmd(args...)
	return cn.writeCmds(req)
}

// Subscribes the client to the specified channels.
func (c *PubSub) Subscribe(channels ...string) error {
	err := c.subscribe("SUBSCRIBE", channels...)
	if err == nil {
		c.channels = append(c.channels, channels...)
	}
	return err
}

// Subscribes the client to the given patterns.
func (c *PubSub) PSubscribe(patterns ...string) error {
	err := c.subscribe("PSUBSCRIBE", patterns...)
	if err == nil {
		c.patterns = append(c.patterns, patterns...)
	}
	return err
}

func remove(ss []string, es ...string) []string {
	if len(es) == 0 {
		return ss[:0]
	}
	for _, e := range es {
		for i, s := range ss {
			if s == e {
				ss = append(ss[:i], ss[i+1:]...)
				break
			}
		}
	}
	return ss
}

// Unsubscribes the client from the given channels, or from all of
// them if none is given.
func (c *PubSub) Unsubscribe(channels ...string) error {
	err := c.subscribe("UNSUBSCRIBE", channels...)
	if err == nil {
		c.channels = remove(c.channels, channels...)
	}
	return err
}

// Unsubscribes the client from the given patterns, or from all of
// them if none is given.
func (c *PubSub) PUnsubscribe(patterns ...string) error {
	err := c.subscribe("PUNSUBSCRIBE", patterns...)
	if err == nil {
		c.patterns = remove(c.patterns, patterns...)
	}
	return err
}

func (c *PubSub) Ping(payload string) error {
	cn, _, err := c.conn()
	if err != nil {
		return err
	}

	args := []interface{}{"PING"}
	if payload != "" {
		args = append(args, payload)
	}
	cmd := NewCmd(args...)
	return cn.writeCmds(cmd)
}

// Message received after a successful subscription to channel.
type Subscription struct {
	// Can be "subscribe", "unsubscribe", "psubscribe" or "punsubscribe".
	Kind string
	// Channel name we have subscribed to.
	Channel string
	// Number of channels we are currently subscribed to.
	Count int
}

func (m *Subscription) String() string {
	return fmt.Sprintf("%s: %s", m.Kind, m.Channel)
}

// Message received as result of a PUBLISH command issued by another client.
type Message struct {
	Channel string
	Pattern string
	Payload string
}

func (m *Message) String() string {
	return fmt.Sprintf("Message<%s: %s>", m.Channel, m.Payload)
}

// TODO: remove PMessage if favor of Message

// Message matching a pattern-matching subscription received as result
// of a PUBLISH command issued by another client.
type PMessage struct {
	Channel string
	Pattern string
	Payload string
}

func (m *PMessage) String() string {
	return fmt.Sprintf("PMessage<%s: %s>", m.Channel, m.Payload)
}

// Pong received as result of a PING command issued by another client.
type Pong struct {
	Payload string
}

func (p *Pong) String() string {
	if p.Payload != "" {
		return fmt.Sprintf("Pong<%s>", p.Payload)
	}
	return "Pong"
}

func newMessage(reply []interface{}) (interface{}, error) {
	switch kind := reply[0].(string); kind {
	case "subscribe", "unsubscribe", "psubscribe", "punsubscribe":
		return &Subscription{
			Kind:    kind,
			Channel: reply[1].(string),
			Count:   int(reply[2].(int64)),
		}, nil
	case "message":
		return &Message{
			Channel: reply[1].(string),
			Payload: reply[2].(string),
		}, nil
	case "pmessage":
		return &PMessage{
			Pattern: reply[1].(string),
			Channel: reply[2].(string),
			Payload: reply[3].(string),
		}, nil
	case "pong":
		return &Pong{
			Payload: reply[1].(string),
		}, nil
	default:
		return nil, fmt.Errorf("redis: unsupported pubsub notification: %q", kind)
	}
}

// ReceiveTimeout acts like Receive but returns an error if message
// is not received in time. This is low-level API and most clients
// should use ReceiveMessage.
func (c *PubSub) ReceiveTimeout(timeout time.Duration) (interface{}, error) {
	cn, _, err := c.conn()
	if err != nil {
		return nil, err
	}
	cn.ReadTimeout = timeout

	cmd := NewSliceCmd()
	err = cmd.readReply(cn)
	c.putConn(cn, err)
	if err != nil {
		return nil, err
	}
	return newMessage(cmd.Val())
}

// Receive returns a message as a Subscription, Message, PMessage,
// Pong or error. See PubSub example for details. This is low-level
// API and most clients should use ReceiveMessage.
func (c *PubSub) Receive() (interface{}, error) {
	return c.ReceiveTimeout(0)
}

func (c *PubSub) reconnect(reason error) {
	// Close current connection.
	c.connPool.(*stickyConnPool).Reset(reason)

	if len(c.channels) > 0 {
		if err := c.Subscribe(c.channels...); err != nil {
			log.Printf("redis: Subscribe failed: %s", err)
		}
	}
	if len(c.patterns) > 0 {
		if err := c.PSubscribe(c.patterns...); err != nil {
			log.Printf("redis: PSubscribe failed: %s", err)
		}
	}
}

// ReceiveMessage returns a message or error. It automatically
// reconnects to Redis in case of network errors.
func (c *PubSub) ReceiveMessage() (*Message, error) {
	var errNum int
	for {
		msgi, err := c.ReceiveTimeout(5 * time.Second)
		if err != nil {
			if !isNetworkError(err) {
				return nil, err
			}

			goodConn := errNum == 0
			errNum++

			if goodConn {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					err := c.Ping("")
					if err == nil {
						continue
					}
					log.Printf("redis: PubSub.Ping failed: %s", err)
				}
			}

			if errNum > 2 {
				time.Sleep(time.Second)
			}
			c.reconnect(err)
			continue
		}

		// Reset error number.
		errNum = 0

		switch msg := msgi.(type) {
		case *Subscription:
			// Ignore.
		case *Pong:
			// Ignore.
		case *Message:
			return msg, nil
		case *PMessage:
			return &Message{
				Channel: msg.Channel,
				Pattern: msg.Pattern,
				Payload: msg.Payload,
			}, nil
		default:
			return nil, fmt.Errorf("redis: unknown message: %T", msgi)
		}
	}
}
