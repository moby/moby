// Copyright (c) 2012, Sean Treadway, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/streadway/amqp

package amqp

import (
	"reflect"
	"sync"
)

// 0      1         3             7                  size+7 size+8
// +------+---------+-------------+  +------------+  +-----------+
// | type | channel |     size    |  |  payload   |  | frame-end |
// +------+---------+-------------+  +------------+  +-----------+
//  octet   short         long         size octets       octet
const frameHeaderSize = 1 + 2 + 4 + 1

/*
Channel represents an AMQP channel. Used as a context for valid message
exchange.  Errors on methods with this Channel as a receiver means this channel
should be discarded and a new channel established.

*/
type Channel struct {
	destructor sync.Once
	sendM      sync.Mutex // sequence channel frames
	m          sync.Mutex // struct field mutex

	connection *Connection

	rpc       chan message
	consumers *consumers

	id uint16

	// true when we will never notify again
	noNotify bool

	// Channel and Connection exceptions will be broadcast on these listeners.
	closes []chan *Error

	// Listeners for active=true flow control.  When true is sent to a listener,
	// publishing should pause until false is sent to listeners.
	flows []chan bool

	// Listeners for returned publishings for unroutable messages on mandatory
	// publishings or undeliverable messages on immediate publishings.
	returns []chan Return

	// Listeners for when the server notifies the client that
	// a consumer has been cancelled.
	cancels []chan string

	// Allocated when in confirm mode in order to track publish counter and order confirms
	confirms   *confirms
	confirming bool

	// Selects on any errors from shutdown during RPC
	errors chan *Error

	// State machine that manages frame order, must only be mutated by the connection
	recv func(*Channel, frame) error

	// State that manages the send behavior after before and after shutdown, must
	// only be mutated in shutdown()
	send func(*Channel, message) error

	// Current state for frame re-assembly, only mutated from recv
	message messageWithContent
	header  *headerFrame
	body    []byte
}

// Constructs a new channel with the given framing rules
func newChannel(c *Connection, id uint16) *Channel {
	return &Channel{
		connection: c,
		id:         id,
		rpc:        make(chan message),
		consumers:  makeConsumers(),
		confirms:   newConfirms(),
		recv:       (*Channel).recvMethod,
		send:       (*Channel).sendOpen,
		errors:     make(chan *Error, 1),
	}
}

// shutdown is called by Connection after the channel has been removed from the
// connection registry.
func (me *Channel) shutdown(e *Error) {
	me.destructor.Do(func() {
		me.m.Lock()
		defer me.m.Unlock()

		// Broadcast abnormal shutdown
		if e != nil {
			for _, c := range me.closes {
				c <- e
			}
		}

		me.send = (*Channel).sendClosed

		// Notify RPC if we're selecting
		if e != nil {
			me.errors <- e
		}

		me.consumers.closeAll()

		for _, c := range me.closes {
			close(c)
		}

		for _, c := range me.flows {
			close(c)
		}

		for _, c := range me.returns {
			close(c)
		}

		for _, c := range me.cancels {
			close(c)
		}

		if me.confirms != nil {
			me.confirms.Close()
		}

		me.noNotify = true
	})
}

func (me *Channel) open() error {
	return me.call(&channelOpen{}, &channelOpenOk{})
}

// Performs a request/response call for when the message is not NoWait and is
// specified as Synchronous.
func (me *Channel) call(req message, res ...message) error {
	if err := me.send(me, req); err != nil {
		return err
	}

	if req.wait() {
		select {
		case e := <-me.errors:
			return e

		case msg := <-me.rpc:
			if msg != nil {
				for _, try := range res {
					if reflect.TypeOf(msg) == reflect.TypeOf(try) {
						// *res = *msg
						vres := reflect.ValueOf(try).Elem()
						vmsg := reflect.ValueOf(msg).Elem()
						vres.Set(vmsg)
						return nil
					}
				}
				return ErrCommandInvalid
			} else {
				// RPC channel has been closed without an error, likely due to a hard
				// error on the Connection.  This indicates we have already been
				// shutdown and if were waiting, will have returned from the errors chan.
				return ErrClosed
			}
		}
	}

	return nil
}

func (me *Channel) sendClosed(msg message) (err error) {
	me.sendM.Lock()
	defer me.sendM.Unlock()

	// After a 'channel.close' is sent or received the only valid response is
	// channel.close-ok
	if _, ok := msg.(*channelCloseOk); ok {
		return me.connection.send(&methodFrame{
			ChannelId: me.id,
			Method:    msg,
		})
	}

	return ErrClosed
}

func (me *Channel) sendOpen(msg message) (err error) {
	me.sendM.Lock()
	defer me.sendM.Unlock()

	if content, ok := msg.(messageWithContent); ok {
		props, body := content.getContent()
		class, _ := content.id()
		size := me.connection.Config.FrameSize - frameHeaderSize

		if err = me.connection.send(&methodFrame{
			ChannelId: me.id,
			Method:    content,
		}); err != nil {
			return
		}

		if err = me.connection.send(&headerFrame{
			ChannelId:  me.id,
			ClassId:    class,
			Size:       uint64(len(body)),
			Properties: props,
		}); err != nil {
			return
		}

		for i, j := 0, size; i < len(body); i, j = j, j+size {
			if j > len(body) {
				j = len(body)
			}

			if err = me.connection.send(&bodyFrame{
				ChannelId: me.id,
				Body:      body[i:j],
			}); err != nil {
				return
			}
		}
	} else {
		err = me.connection.send(&methodFrame{
			ChannelId: me.id,
			Method:    msg,
		})
	}

	return
}

// Eventually called via the state machine from the connection's reader
// goroutine, so assumes serialized access.
func (me *Channel) dispatch(msg message) {
	switch m := msg.(type) {
	case *channelClose:
		me.connection.closeChannel(me, newError(m.ReplyCode, m.ReplyText))
		me.send(me, &channelCloseOk{})

	case *channelFlow:
		for _, c := range me.flows {
			c <- m.Active
		}
		me.send(me, &channelFlowOk{Active: m.Active})

	case *basicCancel:
		for _, c := range me.cancels {
			c <- m.ConsumerTag
		}
		me.send(me, &basicCancelOk{ConsumerTag: m.ConsumerTag})

	case *basicReturn:
		ret := newReturn(*m)
		for _, c := range me.returns {
			c <- *ret
		}

	case *basicAck:
		if me.confirming {
			if m.Multiple {
				me.confirms.Multiple(Confirmation{m.DeliveryTag, true})
			} else {
				me.confirms.One(Confirmation{m.DeliveryTag, true})
			}
		}

	case *basicNack:
		if me.confirming {
			if m.Multiple {
				me.confirms.Multiple(Confirmation{m.DeliveryTag, false})
			} else {
				me.confirms.One(Confirmation{m.DeliveryTag, false})
			}
		}

	case *basicDeliver:
		me.consumers.send(m.ConsumerTag, newDelivery(me, m))
		// TODO log failed consumer and close channel, this can happen when
		// deliveries are in flight and a no-wait cancel has happened

	default:
		me.rpc <- msg
	}
}

func (me *Channel) transition(f func(*Channel, frame) error) error {
	me.recv = f
	return nil
}

func (me *Channel) recvMethod(f frame) error {
	switch frame := f.(type) {
	case *methodFrame:
		if msg, ok := frame.Method.(messageWithContent); ok {
			me.body = make([]byte, 0)
			me.message = msg
			return me.transition((*Channel).recvHeader)
		}

		me.dispatch(frame.Method) // termination state
		return me.transition((*Channel).recvMethod)

	case *headerFrame:
		// drop
		return me.transition((*Channel).recvMethod)

	case *bodyFrame:
		// drop
		return me.transition((*Channel).recvMethod)

	default:
		panic("unexpected frame type")
	}

	panic("unreachable")
}

func (me *Channel) recvHeader(f frame) error {
	switch frame := f.(type) {
	case *methodFrame:
		// interrupt content and handle method
		return me.recvMethod(f)

	case *headerFrame:
		// start collecting if we expect body frames
		me.header = frame

		if frame.Size == 0 {
			me.message.setContent(me.header.Properties, me.body)
			me.dispatch(me.message) // termination state
			return me.transition((*Channel).recvMethod)
		} else {
			return me.transition((*Channel).recvContent)
		}

	case *bodyFrame:
		// drop and reset
		return me.transition((*Channel).recvMethod)

	default:
		panic("unexpected frame type")
	}

	panic("unreachable")
}

// state after method + header and before the length
// defined by the header has been reached
func (me *Channel) recvContent(f frame) error {
	switch frame := f.(type) {
	case *methodFrame:
		// interrupt content and handle method
		return me.recvMethod(f)

	case *headerFrame:
		// drop and reset
		return me.transition((*Channel).recvMethod)

	case *bodyFrame:
		me.body = append(me.body, frame.Body...)

		if uint64(len(me.body)) >= me.header.Size {
			me.message.setContent(me.header.Properties, me.body)
			me.dispatch(me.message) // termination state
			return me.transition((*Channel).recvMethod)
		}

		return me.transition((*Channel).recvContent)

	default:
		panic("unexpected frame type")
	}

	panic("unreachable")
}

/*
Close initiate a clean channel closure by sending a close message with the error
code set to '200'.

It is safe to call this method multiple times.

*/
func (me *Channel) Close() error {
	defer me.connection.closeChannel(me, nil)
	return me.call(
		&channelClose{ReplyCode: replySuccess},
		&channelCloseOk{},
	)
}

/*
NotifyClose registers a listener for when the server sends a channel or
connection exception in the form of a Connection.Close or Channel.Close method.
Connection exceptions will be broadcast to all open channels and all channels
will be closed, where channel exceptions will only be broadcast to listeners to
this channel.

The chan provided will be closed when the Channel is closed and on a
graceful close, no error will be sent.

*/
func (me *Channel) NotifyClose(c chan *Error) chan *Error {
	me.m.Lock()
	defer me.m.Unlock()

	if me.noNotify {
		close(c)
	} else {
		me.closes = append(me.closes, c)
	}

	return c
}

/*
NotifyFlow registers a listener for basic.flow methods sent by the server.
When `true` is sent on one of the listener channels, all publishers should
pause until a `false` is sent.

The server may ask the producer to pause or restart the flow of Publishings
sent by on a channel. This is a simple flow-control mechanism that a server can
use to avoid overflowing its queues or otherwise finding itself receiving more
messages than it can process. Note that this method is not intended for window
control. It does not affect contents returned by basic.get-ok methods.

When a new channel is opened, it is active (flow is active). Some
applications assume that channels are inactive until started. To emulate
this behavior a client MAY open the channel, then pause it.

Publishers should respond to a flow messages as rapidly as possible and the
server may disconnect over producing channels that do not respect these
messages.

basic.flow-ok methods will always be returned to the server regardless of
the number of listeners there are.

To control the flow of deliveries from the server.  Use the Channel.Flow()
method instead.

Note: RabbitMQ will rather use TCP pushback on the network connection instead
of sending basic.flow.  This means that if a single channel is producing too
much on the same connection, all channels using that connection will suffer,
including acknowledgments from deliveries.  Use different Connections if you
desire to interleave consumers and producers in the same process to avoid your
basic.ack messages from getting rate limited with your basic.publish messages.

*/
func (me *Channel) NotifyFlow(c chan bool) chan bool {
	me.m.Lock()
	defer me.m.Unlock()

	if me.noNotify {
		close(c)
	} else {
		me.flows = append(me.flows, c)
	}

	return c
}

/*
NotifyReturn registers a listener for basic.return methods.  These can be sent
from the server when a publish is undeliverable either from the mandatory or
immediate flags.

A return struct has a copy of the Publishing along with some error
information about why the publishing failed.

*/
func (me *Channel) NotifyReturn(c chan Return) chan Return {
	me.m.Lock()
	defer me.m.Unlock()

	if me.noNotify {
		close(c)
	} else {
		me.returns = append(me.returns, c)
	}

	return c
}

/*
NotifyCancel registers a listener for basic.cancel methods.  These can be sent
from the server when a queue is deleted or when consuming from a mirrored queue
where the master has just failed (and was moved to another node)

The subscription tag is returned to the listener.

*/
func (me *Channel) NotifyCancel(c chan string) chan string {
	me.m.Lock()
	defer me.m.Unlock()

	if me.noNotify {
		close(c)
	} else {
		me.cancels = append(me.cancels, c)
	}

	return c
}

/*
NotifyConfirm calls NotifyPublish and starts a goroutines sending
ordered Ack and Nack DeliveryTag to the respective channels.

For strict ordering, use NotifyPublish instead.
*/
func (me *Channel) NotifyConfirm(ack, nack chan uint64) (chan uint64, chan uint64) {
	confirms := me.NotifyPublish(make(chan Confirmation, len(ack)+len(nack)))

	go func() {
		for c := range confirms {
			if c.Ack {
				ack <- c.DeliveryTag
			} else {
				nack <- c.DeliveryTag
			}
		}
		close(ack)
		if nack != ack {
			close(nack)
		}
	}()

	return ack, nack
}

/*
NotifyPublish registers a listener for reliable publishing. Receives from this
chan for every publish after Channel.Confirm will be in order starting with
DeliveryTag 1.

There will be one and only one Confimration Publishing starting with the
delviery tag of 1 and progressing sequentially until the total number of
Publishings have been seen by the server.

Acknowledgments will be received in the order of delivery from the
NotifyPublish channels even if the server acknowledges them out of order.

The listener chan will be closed when the Channel is closed.

The capacity of the chan Confirmation must be at least as large as the
number of outstanding publishings.  Not having enough buffered chans will
create a deadlock if you attempt to perform other operations on the Connection
or Channel while confirms are in-flight.

It's advisable to wait for all Confirmations to arrive before calling
Channel.Close() or Connection.Close().

*/
func (me *Channel) NotifyPublish(confirm chan Confirmation) chan Confirmation {
	me.m.Lock()
	defer me.m.Unlock()

	if me.noNotify {
		close(confirm)
	} else {
		me.confirms.Listen(confirm)
	}

	return confirm

}

/*
Qos controls how many messages or how many bytes the server will try to keep on
the network for consumers before receiving delivery acks.  The intent of Qos is
to make sure the network buffers stay full between the server and client.

With a prefetch count greater than zero, the server will deliver that many
messages to consumers before acknowledgments are received.  The server ignores
this option when consumers are started with noAck because no acknowledgments
are expected or sent.

With a prefetch size greater than zero, the server will try to keep at least
that many bytes of deliveries flushed to the network before receiving
acknowledgments from the consumers.  This option is ignored when consumers are
started with noAck.

When global is true, these Qos settings apply to all existing and future
consumers on all channels on the same connection.  When false, the Channel.Qos
settings will apply to all existing and future consumers on this channel.
RabbitMQ does not implement the global flag.

To get round-robin behavior between consumers consuming from the same queue on
different connections, set the prefetch count to 1, and the next available
message on the server will be delivered to the next available consumer.

If your consumer work time is reasonably consistent and not much greater
than two times your network round trip time, you will see significant
throughput improvements starting with a prefetch count of 2 or slightly
greater as described by benchmarks on RabbitMQ.

http://www.rabbitmq.com/blog/2012/04/25/rabbitmq-performance-measurements-part-2/
*/
func (me *Channel) Qos(prefetchCount, prefetchSize int, global bool) error {
	return me.call(
		&basicQos{
			PrefetchCount: uint16(prefetchCount),
			PrefetchSize:  uint32(prefetchSize),
			Global:        global,
		},
		&basicQosOk{},
	)
}

/*
Cancel stops deliveries to the consumer chan established in Channel.Consume and
identified by consumer.

Only use this method to cleanly stop receiving deliveries from the server and
cleanly shut down the consumer chan identified by this tag.  Using this method
and waiting for remaining messages to flush from the consumer chan will ensure
all messages received on the network will be delivered to the receiver of your
consumer chan.

Continue consuming from the chan Delivery provided by Channel.Consume until the
chan closes.

When noWait is true, do not wait for the server to acknowledge the cancel.
Only use this when you are certain there are no deliveries requiring
acknowledgment are in-flight otherwise they will arrive and be dropped in the
client without an ack and will not be redelivered to other consumers.

*/
func (me *Channel) Cancel(consumer string, noWait bool) error {
	req := &basicCancel{
		ConsumerTag: consumer,
		NoWait:      noWait,
	}
	res := &basicCancelOk{}

	if err := me.call(req, res); err != nil {
		return err
	}

	if req.wait() {
		me.consumers.close(res.ConsumerTag)
	} else {
		// Potentially could drop deliveries in flight
		me.consumers.close(consumer)
	}

	return nil
}

/*
QueueDeclare declares a queue to hold messages and deliver to consumers.
Declaring creates a queue if it doesn't already exist, or ensures that an
existing queue matches the same parameters.

Every queue declared gets a default binding to the empty exchange "" which has
the type "direct" with the routing key matching the queue's name.  With this
default binding, it is possible to publish messages that route directly to
this queue by publishing to "" with the routing key of the queue name.

  QueueDeclare("alerts", true, false, false false, false, nil)
  Publish("", "alerts", false, false, Publishing{Body: []byte("...")})

  Delivery       Exchange  Key       Queue
  -----------------------------------------------
  key: alerts -> ""     -> alerts -> alerts

The queue name may be empty, in which the server will generate a unique name
which will be returned in the Name field of Queue struct.

Durable and Non-Auto-Deleted queues will survive server restarts and remain
when there are no remaining consumers or bindings.  Persistent publishings will
be restored in this queue on server restart.  These queues are only able to be
bound to durable exchanges.

Non-Durable and Auto-Deleted queues will not be redeclared on server restart
and will be deleted by the server after a short time when the last consumer is
canceled or the last consumer's channel is closed.  Queues with this lifetime
can also be deleted normally with QueueDelete.  These durable queues can only
be bound to non-durable exchanges.

Non-Durable and Non-Auto-Deleted queues will remain declared as long as the
server is running regardless of how many consumers.  This lifetime is useful
for temporary topologies that may have long delays between consumer activity.
These queues can only be bound to non-durable exchanges.

Durable and Auto-Deleted queues will be restored on server restart, but without
active consumers, will not survive and be removed.  This Lifetime is unlikely
to be useful.

Exclusive queues are only accessible by the connection that declares them and
will be deleted when the connection closes.  Channels on other connections
will receive an error when attempting declare, bind, consume, purge or delete a
queue with the same name.

When noWait is true, the queue will assume to be declared on the server.  A
channel exception will arrive if the conditions are met for existing queues
or attempting to modify an existing queue from a different connection.

When the error return value is not nil, you can assume the queue could not be
declared with these parameters and the channel will be closed.

*/
func (me *Channel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args Table) (Queue, error) {
	if err := args.Validate(); err != nil {
		return Queue{}, err
	}

	req := &queueDeclare{
		Queue:      name,
		Passive:    false,
		Durable:    durable,
		AutoDelete: autoDelete,
		Exclusive:  exclusive,
		NoWait:     noWait,
		Arguments:  args,
	}
	res := &queueDeclareOk{}

	if err := me.call(req, res); err != nil {
		return Queue{}, err
	}

	if req.wait() {
		return Queue{
			Name:      res.Queue,
			Messages:  int(res.MessageCount),
			Consumers: int(res.ConsumerCount),
		}, nil
	}

	return Queue{
		Name: name,
	}, nil

	panic("unreachable")
}

/*

QueueDeclarePassive is functionally and parametrically equivalent to
QueueDeclare, except that it sets the "passive" attribute to true. A passive
queue is assumed by RabbitMQ to already exist, and attempting to connect to a
non-existent queue will cause RabbitMQ to throw an exception. This function
can be used to test for the existence of a queue.

*/
func (me *Channel) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args Table) (Queue, error) {
	if err := args.Validate(); err != nil {
		return Queue{}, err
	}

	req := &queueDeclare{
		Queue:      name,
		Passive:    true,
		Durable:    durable,
		AutoDelete: autoDelete,
		Exclusive:  exclusive,
		NoWait:     noWait,
		Arguments:  args,
	}
	res := &queueDeclareOk{}

	if err := me.call(req, res); err != nil {
		return Queue{}, err
	}

	if req.wait() {
		return Queue{
			Name:      res.Queue,
			Messages:  int(res.MessageCount),
			Consumers: int(res.ConsumerCount),
		}, nil
	}

	return Queue{
		Name: name,
	}, nil

	panic("unreachable")
}

/*
QueueInspect passively declares a queue by name to inspect the current message
count, consumer count.

Use this method to check how many unacknowledged messages reside in the queue
and how many consumers are receiving deliveries and whether a queue by this
name already exists.

If the queue by this name exists, use Channel.QueueDeclare check if it is
declared with specific parameters.

If a queue by this name does not exist, an error will be returned and the
channel will be closed.

*/
func (me *Channel) QueueInspect(name string) (Queue, error) {
	req := &queueDeclare{
		Queue:   name,
		Passive: true,
	}
	res := &queueDeclareOk{}

	err := me.call(req, res)

	state := Queue{
		Name:      name,
		Messages:  int(res.MessageCount),
		Consumers: int(res.ConsumerCount),
	}

	return state, err
}

/*
QueueBind binds an exchange to a queue so that publishings to the exchange will
be routed to the queue when the publishing routing key matches the binding
routing key.

  QueueBind("pagers", "alert", "log", false, nil)
  QueueBind("emails", "info", "log", false, nil)

  Delivery       Exchange  Key       Queue
  -----------------------------------------------
  key: alert --> log ----> alert --> pagers
  key: info ---> log ----> info ---> emails
  key: debug --> log       (none)    (dropped)

If a binding with the same key and arguments already exists between the
exchange and queue, the attempt to rebind will be ignored and the existing
binding will be retained.

In the case that multiple bindings may cause the message to be routed to the
same queue, the server will only route the publishing once.  This is possible
with topic exchanges.

  QueueBind("pagers", "alert", "amq.topic", false, nil)
  QueueBind("emails", "info", "amq.topic", false, nil)
  QueueBind("emails", "#", "amq.topic", false, nil) // match everything

  Delivery       Exchange        Key       Queue
  -----------------------------------------------
  key: alert --> amq.topic ----> alert --> pagers
  key: info ---> amq.topic ----> # ------> emails
                           \---> info ---/
  key: debug --> amq.topic ----> # ------> emails

It is only possible to bind a durable queue to a durable exchange regardless of
whether the queue or exchange is auto-deleted.  Bindings between durable queues
and exchanges will also be restored on server restart.

If the binding could not complete, an error will be returned and the channel
will be closed.

When noWait is true and the queue could not be bound, the channel will be
closed with an error.

*/
func (me *Channel) QueueBind(name, key, exchange string, noWait bool, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&queueBind{
			Queue:      name,
			Exchange:   exchange,
			RoutingKey: key,
			NoWait:     noWait,
			Arguments:  args,
		},
		&queueBindOk{},
	)
}

/*
QueueUnbind removes a binding between an exchange and queue matching the key and
arguments.

It is possible to send and empty string for the exchange name which means to
unbind the queue from the default exchange.

*/
func (me *Channel) QueueUnbind(name, key, exchange string, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&queueUnbind{
			Queue:      name,
			Exchange:   exchange,
			RoutingKey: key,
			Arguments:  args,
		},
		&queueUnbindOk{},
	)
}

/*
QueuePurge removes all messages from the named queue which are not waiting to
be acknowledged.  Messages that have been delivered but have not yet been
acknowledged will not be removed.

When successful, returns the number of messages purged.

If noWait is true, do not wait for the server response and the number of
messages purged will not be meaningful.
*/
func (me *Channel) QueuePurge(name string, noWait bool) (int, error) {
	req := &queuePurge{
		Queue:  name,
		NoWait: noWait,
	}
	res := &queuePurgeOk{}

	err := me.call(req, res)

	return int(res.MessageCount), err
}

/*
QueueDelete removes the queue from the server including all bindings then
purges the messages based on server configuration, returning the number of
messages purged.

When ifUnused is true, the queue will not be deleted if there are any
consumers on the queue.  If there are consumers, an error will be returned and
the channel will be closed.

When ifEmpty is true, the queue will not be deleted if there are any messages
remaining on the queue.  If there are messages, an error will be returned and
the channel will be closed.

When noWait is true, the queue will be deleted without waiting for a response
from the server.  The purged message count will not be meaningful. If the queue
could not be deleted, a channel exception will be raised and the channel will
be closed.

*/
func (me *Channel) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {
	req := &queueDelete{
		Queue:    name,
		IfUnused: ifUnused,
		IfEmpty:  ifEmpty,
		NoWait:   noWait,
	}
	res := &queueDeleteOk{}

	err := me.call(req, res)

	return int(res.MessageCount), err
}

/*
Consume immediately starts delivering queued messages.

Begin receiving on the returned chan Delivery before any other operation on the
Connection or Channel.

Continues deliveries to the returned chan Delivery until Channel.Cancel,
Connection.Close, Channel.Close, or an AMQP exception occurs.  Consumers must
range over the chan to ensure all deliveries are received.  Unreceived
deliveries will block all methods on the same connection.

All deliveries in AMQP must be acknowledged.  It is expected of the consumer to
call Delivery.Ack after it has successfully processed the delivery.  If the
consumer is cancelled or the channel or connection is closed any unacknowledged
deliveries will be requeued at the end of the same queue.

The consumer is identified by a string that is unique and scoped for all
consumers on this channel.  If you wish to eventually cancel the consumer, use
the same non-empty idenfitier in Channel.Cancel.  An empty string will cause
the library to generate a unique identity.  The consumer identity will be
included in every Delivery in the ConsumerTag field

When autoAck (also known as noAck) is true, the server will acknowledge
deliveries to this consumer prior to writing the delivery to the network.  When
autoAck is true, the consumer should not call Delivery.Ack.  Automatically
acknowledging deliveries means that some deliveries may get lost if the
consumer is unable to process them after the server delivers them.

When exclusive is true, the server will ensure that this is the sole consumer
from this queue.  When exclusive is false, the server will fairly distribute
deliveries across multiple consumers.

When noLocal is true, the server will not deliver publishing sent from the same
connection to this consumer.  It's advisable to use separate connections for
Channel.Publish and Channel.Consume so not to have TCP pushback on publishing
affect the ability to consume messages, so this parameter is here mostly for
completeness.

When noWait is true, do not wait for the server to confirm the request and
immediately begin deliveries.  If it is not possible to consume, a channel
exception will be raised and the channel will be closed.

Optional arguments can be provided that have specific semantics for the queue
or server.

When the channel or connection closes, all delivery chans will also close.

Deliveries on the returned chan will be buffered indefinitely.  To limit memory
of this buffer, use the Channel.Qos method to limit the amount of
unacknowledged/buffered deliveries the server will deliver on this Channel.

*/
func (me *Channel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args Table) (<-chan Delivery, error) {
	// When we return from me.call, there may be a delivery already for the
	// consumer that hasn't been added to the consumer hash yet.  Because of
	// this, we never rely on the server picking a consumer tag for us.

	if err := args.Validate(); err != nil {
		return nil, err
	}

	if consumer == "" {
		consumer = uniqueConsumerTag()
	}

	req := &basicConsume{
		Queue:       queue,
		ConsumerTag: consumer,
		NoLocal:     noLocal,
		NoAck:       autoAck,
		Exclusive:   exclusive,
		NoWait:      noWait,
		Arguments:   args,
	}
	res := &basicConsumeOk{}

	deliveries := make(chan Delivery)

	me.consumers.add(consumer, deliveries)

	if err := me.call(req, res); err != nil {
		me.consumers.close(consumer)
		return nil, err
	}

	return (<-chan Delivery)(deliveries), nil
}

/*
ExchangeDeclare declares an exchange on the server. If the exchange does not
already exist, the server will create it.  If the exchange exists, the server
verifies that it is of the provided type, durability and auto-delete flags.

Errors returned from this method will close the channel.

Exchange names starting with "amq." are reserved for pre-declared and
standardized exchanges. The client MAY declare an exchange starting with
"amq." if the passive option is set, or the exchange already exists.  Names can
consists of a non-empty sequence of letters, digits, hyphen, underscore,
period, or colon.

Each exchange belongs to one of a set of exchange kinds/types implemented by
the server. The exchange types define the functionality of the exchange - i.e.
how messages are routed through it. Once an exchange is declared, its type
cannot be changed.  The common types are "direct", "fanout", "topic" and
"headers".

Durable and Non-Auto-Deleted exchanges will survive server restarts and remain
declared when there are no remaining bindings.  This is the best lifetime for
long-lived exchange configurations like stable routes and default exchanges.

Non-Durable and Auto-Deleted exchanges will be deleted when there are no
remaining bindings and not restored on server restart.  This lifetime is
useful for temporary topologies that should not pollute the virtual host on
failure or after the consumers have completed.

Non-Durable and Non-Auto-deleted exchanges will remain as long as the server is
running including when there are no remaining bindings.  This is useful for
temporary topologies that may have long delays between bindings.

Durable and Auto-Deleted exchanges will survive server restarts and will be
removed before and after server restarts when there are no remaining bindings.
These exchanges are useful for robust temporary topologies or when you require
binding durable queues to auto-deleted exchanges.

Note: RabbitMQ declares the default exchange types like 'amq.fanout' as
durable, so queues that bind to these pre-declared exchanges must also be
durable.

Exchanges declared as `internal` do not accept accept publishings. Internal
exchanges are useful for when you wish to implement inter-exchange topologies
that should not be exposed to users of the broker.

When noWait is true, declare without waiting for a confirmation from the server.
The channel may be closed as a result of an error.  Add a NotifyClose listener
to respond to any exceptions.

Optional amqp.Table of arguments that are specific to the server's implementation of
the exchange can be sent for exchange types that require extra parameters.
*/
func (me *Channel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&exchangeDeclare{
			Exchange:   name,
			Type:       kind,
			Passive:    false,
			Durable:    durable,
			AutoDelete: autoDelete,
			Internal:   internal,
			NoWait:     noWait,
			Arguments:  args,
		},
		&exchangeDeclareOk{},
	)
}

/*

ExchangeDeclarePassive is functionally and parametrically equivalent to
ExchangeDeclare, except that it sets the "passive" attribute to true. A passive
exchange is assumed by RabbitMQ to already exist, and attempting to connect to a
non-existent exchange will cause RabbitMQ to throw an exception. This function
can be used to detect the existence of an exchange.

*/
func (me *Channel) ExchangeDeclarePassive(name, kind string, durable, autoDelete, internal, noWait bool, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&exchangeDeclare{
			Exchange:   name,
			Type:       kind,
			Passive:    true,
			Durable:    durable,
			AutoDelete: autoDelete,
			Internal:   internal,
			NoWait:     noWait,
			Arguments:  args,
		},
		&exchangeDeclareOk{},
	)
}

/*
ExchangeDelete removes the named exchange from the server. When an exchange is
deleted all queue bindings on the exchange are also deleted.  If this exchange
does not exist, the channel will be closed with an error.

When ifUnused is true, the server will only delete the exchange if it has no queue
bindings.  If the exchange has queue bindings the server does not delete it
but close the channel with an exception instead.  Set this to true if you are
not the sole owner of the exchange.

When noWait is true, do not wait for a server confirmation that the exchange has
been deleted.  Failing to delete the channel could close the channel.  Add a
NotifyClose listener to respond to these channel exceptions.
*/
func (me *Channel) ExchangeDelete(name string, ifUnused, noWait bool) error {
	return me.call(
		&exchangeDelete{
			Exchange: name,
			IfUnused: ifUnused,
			NoWait:   noWait,
		},
		&exchangeDeleteOk{},
	)
}

/*
ExchangeBind binds an exchange to another exchange to create inter-exchange
routing topologies on the server.  This can decouple the private topology and
routing exchanges from exchanges intended solely for publishing endpoints.

Binding two exchanges with identical arguments will not create duplicate
bindings.

Binding one exchange to another with multiple bindings will only deliver a
message once.  For example if you bind your exchange to `amq.fanout` with two
different binding keys, only a single message will be delivered to your
exchange even though multiple bindings will match.

Given a message delivered to the source exchange, the message will be forwarded
to the destination exchange when the routing key is matched.

  ExchangeBind("sell", "MSFT", "trade", false, nil)
  ExchangeBind("buy", "AAPL", "trade", false, nil)

  Delivery       Source      Key      Destination
  example        exchange             exchange
  -----------------------------------------------
  key: AAPL  --> trade ----> MSFT     sell
                       \---> AAPL --> buy

When noWait is true, do not wait for the server to confirm the binding.  If any
error occurs the channel will be closed.  Add a listener to NotifyClose to
handle these errors.

Optional arguments specific to the exchanges bound can also be specified.
*/
func (me *Channel) ExchangeBind(destination, key, source string, noWait bool, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&exchangeBind{
			Destination: destination,
			Source:      source,
			RoutingKey:  key,
			NoWait:      noWait,
			Arguments:   args,
		},
		&exchangeBindOk{},
	)
}

/*
ExchangeUnbind unbinds the destination exchange from the source exchange on the
server by removing the routing key between them.  This is the inverse of
ExchangeBind.  If the binding does not currently exist, an error will be
returned.

When noWait is true, do not wait for the server to confirm the deletion of the
binding.  If any error occurs the channel will be closed.  Add a listener to
NotifyClose to handle these errors.

Optional arguments that are specific to the type of exchanges bound can also be
provided.  These must match the same arguments specified in ExchangeBind to
identify the binding.
*/
func (me *Channel) ExchangeUnbind(destination, key, source string, noWait bool, args Table) error {
	if err := args.Validate(); err != nil {
		return err
	}

	return me.call(
		&exchangeUnbind{
			Destination: destination,
			Source:      source,
			RoutingKey:  key,
			NoWait:      noWait,
			Arguments:   args,
		},
		&exchangeUnbindOk{},
	)
}

/*
Publish sends a Publishing from the client to an exchange on the server.

When you want a single message to be delivered to a single queue, you can
publish to the default exchange with the routingKey of the queue name.  This is
because every declared queue gets an implicit route to the default exchange.

Since publishings are asynchronous, any undeliverable message will get returned
by the server.  Add a listener with Channel.NotifyReturn to handle any
undeliverable message when calling publish with either the mandatory or
immediate parameters as true.

Publishings can be undeliverable when the mandatory flag is true and no queue is
bound that matches the routing key, or when the immediate flag is true and no
consumer on the matched queue is ready to accept the delivery.

This can return an error when the channel, connection or socket is closed.  The
error or lack of an error does not indicate whether the server has received this
publishing.

It is possible for publishing to not reach the broker if the underlying socket
is shutdown without pending publishing packets being flushed from the kernel
buffers.  The easy way of making it probable that all publishings reach the
server is to always call Connection.Close before terminating your publishing
application.  The way to ensure that all publishings reach the server is to add
a listener to Channel.NotifyPublish and put the channel in confirm mode with
Channel.Confirm.  Publishing delivery tags and their corresponding
confirmations start at 1.  Exit when all publishings are confirmed.

When Publish does not return an error and the channel is in confirm mode, the
internal counter for DeliveryTags with the first confirmation starting at 1.

*/
func (me *Channel) Publish(exchange, key string, mandatory, immediate bool, msg Publishing) error {
	if err := msg.Headers.Validate(); err != nil {
		return err
	}

	me.m.Lock()
	defer me.m.Unlock()

	if err := me.send(me, &basicPublish{
		Exchange:   exchange,
		RoutingKey: key,
		Mandatory:  mandatory,
		Immediate:  immediate,
		Body:       msg.Body,
		Properties: properties{
			Headers:         msg.Headers,
			ContentType:     msg.ContentType,
			ContentEncoding: msg.ContentEncoding,
			DeliveryMode:    msg.DeliveryMode,
			Priority:        msg.Priority,
			CorrelationId:   msg.CorrelationId,
			ReplyTo:         msg.ReplyTo,
			Expiration:      msg.Expiration,
			MessageId:       msg.MessageId,
			Timestamp:       msg.Timestamp,
			Type:            msg.Type,
			UserId:          msg.UserId,
			AppId:           msg.AppId,
		},
	}); err != nil {
		return err
	}

	if me.confirming {
		me.confirms.Publish()
	}

	return nil
}

/*
Get synchronously receives a single Delivery from the head of a queue from the
server to the client.  In almost all cases, using Channel.Consume will be
preferred.

If there was a delivery waiting on the queue and that delivery was received the
second return value will be true.  If there was no delivery waiting or an error
occured, the ok bool will be false.

All deliveries must be acknowledged including those from Channel.Get.  Call
Delivery.Ack on the returned delivery when you have fully processed this
delivery.

When autoAck is true, the server will automatically acknowledge this message so
you don't have to.  But if you are unable to fully process this message before
the channel or connection is closed, the message will not get requeued.

*/
func (me *Channel) Get(queue string, autoAck bool) (msg Delivery, ok bool, err error) {
	req := &basicGet{Queue: queue, NoAck: autoAck}
	res := &basicGetOk{}
	empty := &basicGetEmpty{}

	if err := me.call(req, res, empty); err != nil {
		return Delivery{}, false, err
	}

	if res.DeliveryTag > 0 {
		return *(newDelivery(me, res)), true, nil
	}

	return Delivery{}, false, nil
}

/*
Tx puts the channel into transaction mode on the server.  All publishings and
acknowledgments following this method will be atomically committed or rolled
back for a single queue.  Call either Channel.TxCommit or Channel.TxRollback to
leave a this transaction and immediately start a new transaction.

The atomicity across multiple queues is not defined as queue declarations and
bindings are not included in the transaction.

The behavior of publishings that are delivered as mandatory or immediate while
the channel is in a transaction is not defined.

Once a channel has been put into transaction mode, it cannot be taken out of
transaction mode.  Use a different channel for non-transactional semantics.

*/
func (me *Channel) Tx() error {
	return me.call(
		&txSelect{},
		&txSelectOk{},
	)
}

/*
TxCommit atomically commits all publishings and acknowledgments for a single
queue and immediately start a new transaction.

Calling this method without having called Channel.Tx is an error.

*/
func (me *Channel) TxCommit() error {
	return me.call(
		&txCommit{},
		&txCommitOk{},
	)
}

/*
TxRollback atomically rolls back all publishings and acknowledgments for a
single queue and immediately start a new transaction.

Calling this method without having called Channel.Tx is an error.

*/
func (me *Channel) TxRollback() error {
	return me.call(
		&txRollback{},
		&txRollbackOk{},
	)
}

/*
Flow pauses the delivery of messages to consumers on this channel.  Channels
are opened with flow control not active, to open a channel with paused
deliveries immediately call this method with true after calling
Connection.Channel.

When active is true, this method asks the server to temporarily pause deliveries
until called again with active as false.

Channel.Get methods will not be affected by flow control.

This method is not intended to act as window control.  Use Channel.Qos to limit
the number of unacknowledged messages or bytes in flight instead.

The server may also send us flow methods to throttle our publishings.  A well
behaving publishing client should add a listener with Channel.NotifyFlow and
pause its publishings when true is sent on that channel.

Note: RabbitMQ prefers to use TCP push back to control flow for all channels on
a connection, so under high volume scenarios, it's wise to open separate
Connections for publishings and deliveries.

*/
func (me *Channel) Flow(active bool) error {
	return me.call(
		&channelFlow{Active: active},
		&channelFlowOk{},
	)
}

/*
Confirm puts this channel into confirm mode so that the client can ensure all
publishings have successfully been received by the server.  After entering this
mode, the server will send a basic.ack or basic.nack message with the deliver
tag set to a 1 based incrementing index corresponding to every publishing
received after the this method returns.

Add a listener to Channel.NotifyPublish to respond to the Confirmations. If
Channel.NotifyPublish is not called, the Confirmations will be silently
ignored.

The order of acknowledgments is not bound to the order of deliveries.

Ack and Nack confirmations will arrive at some point in the future.

Unroutable mandatory or immediate messages are acknowledged immediately after
any Channel.NotifyReturn listeners have been notified.  Other messages are
acknowledged when all queues that should have the message routed to them have
either have received acknowledgment of delivery or have enqueued the message,
persisting the message if necessary.

When noWait is true, the client will not wait for a response.  A channel
exception could occur if the server does not support this method.

*/
func (me *Channel) Confirm(noWait bool) error {
	me.m.Lock()
	defer me.m.Unlock()

	if err := me.call(
		&confirmSelect{Nowait: noWait},
		&confirmSelectOk{},
	); err != nil {
		return err
	}

	me.confirming = true

	return nil
}

/*
Recover redelivers all unacknowledged deliveries on this channel.

When requeue is false, messages will be redelivered to the original consumer.

When requeue is true, messages will be redelivered to any available consumer,
potentially including the original.

If the deliveries cannot be recovered, an error will be returned and the channel
will be closed.

Note: this method is not implemented on RabbitMQ, use Delivery.Nack instead
*/
func (me *Channel) Recover(requeue bool) error {
	return me.call(
		&basicRecover{Requeue: requeue},
		&basicRecoverOk{},
	)
}

/*
Ack acknowledges a delivery by its delivery tag when having been consumed with
Channel.Consume or Channel.Get.

Ack acknowledges all message received prior to the delivery tag when multiple
is true.

See also Delivery.Ack
*/
func (me *Channel) Ack(tag uint64, multiple bool) error {
	return me.send(me, &basicAck{
		DeliveryTag: tag,
		Multiple:    multiple,
	})
}

/*
Nack negatively acknowledges a delivery by its delivery tag.  Prefer this
method to notify the server that you were not able to process this delivery and
it must be redelivered or dropped.

See also Delivery.Nack
*/
func (me *Channel) Nack(tag uint64, multiple bool, requeue bool) error {
	return me.send(me, &basicNack{
		DeliveryTag: tag,
		Multiple:    multiple,
		Requeue:     requeue,
	})
}

/*
Reject negatively acknowledges a delivery by its delivery tag.  Prefer Nack
over Reject when communicating with a RabbitMQ server because you can Nack
multiple messages, reducing the amount of protocol messages to exchange.

See also Delivery.Reject
*/
func (me *Channel) Reject(tag uint64, requeue bool) error {
	return me.send(me, &basicReject{
		DeliveryTag: tag,
		Requeue:     requeue,
	})
}
