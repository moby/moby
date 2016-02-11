package amqp

import "sync"

// confirms resequences and notifies one or multiple publisher confirmation listeners
type confirms struct {
	m         sync.Mutex
	listeners []chan Confirmation
	sequencer map[uint64]Confirmation
	published uint64
	expecting uint64
}

// newConfirms allocates a confirms
func newConfirms() *confirms {
	return &confirms{
		sequencer: map[uint64]Confirmation{},
		published: 0,
		expecting: 1,
	}
}

func (c *confirms) Listen(l chan Confirmation) {
	c.m.Lock()
	defer c.m.Unlock()

	c.listeners = append(c.listeners, l)
}

// publish increments the publishing counter
func (c *confirms) Publish() uint64 {
	c.m.Lock()
	defer c.m.Unlock()

	c.published++
	return c.published
}

// confirm confirms one publishing, increments the expecting delivery tag, and
// removes bookkeeping for that delivery tag.
func (c *confirms) confirm(confirmation Confirmation) {
	delete(c.sequencer, c.expecting)
	c.expecting++
	for _, l := range c.listeners {
		l <- confirmation
	}
}

// resequence confirms any out of order delivered confirmations
func (c *confirms) resequence() {
	for c.expecting <= c.published {
		sequenced, found := c.sequencer[c.expecting]
		if !found {
			return
		}
		c.confirm(sequenced)
	}
}

// one confirms one publishing and all following in the publishing sequence
func (c *confirms) One(confirmed Confirmation) {
	c.m.Lock()
	defer c.m.Unlock()

	if c.expecting == confirmed.DeliveryTag {
		c.confirm(confirmed)
	} else {
		c.sequencer[confirmed.DeliveryTag] = confirmed
	}
	c.resequence()
}

// multiple confirms all publishings up until the delivery tag
func (c *confirms) Multiple(confirmed Confirmation) {
	c.m.Lock()
	defer c.m.Unlock()

	for c.expecting <= confirmed.DeliveryTag {
		c.confirm(Confirmation{c.expecting, confirmed.Ack})
	}
}

// Close closes all listeners, discarding any out of sequence confirmations
func (c *confirms) Close() error {
	c.m.Lock()
	defer c.m.Unlock()

	for _, l := range c.listeners {
		close(l)
	}
	c.listeners = nil
	return nil
}
