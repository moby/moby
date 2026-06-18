package http

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/smithy-go"
	smithysync "github.com/aws/smithy-go/sync"
)

// EventStreamWriter writes events to a stream using a ClientProtocol.
//
// The writer manages a background goroutine that facilitates the write loop.
// Calls to Send() on a writer will block until the message has been written.
//
// The writer doesn't know anything about signing. If event stream messages are
// getting signed by the client then the underlying io.Writer has already been
// wrapped to handle that at this point.
type EventStreamWriter struct {
	protocol ClientProtocol
	schema   *smithy.Schema

	eventStream io.WriteCloser
	stream      chan singleflight
	done        chan struct{}
	err         *smithysync.OnceErr

	closeOnce sync.Once
}

// we send one message at a time, the underlying write loop marshals these into
// the writer and reports back any error to the error channel
type singleflight struct {
	variant *smithy.Schema
	event   smithy.Serializable
	errCh   chan<- error
}

// NewEventStreamWriter returns an EventStreamWriter for the given schema.
func NewEventStreamWriter(protocol ClientProtocol, schema *smithy.Schema, stream io.WriteCloser) *EventStreamWriter {
	w := &EventStreamWriter{
		protocol: protocol,
		schema:   schema,

		eventStream: stream,
		stream:      make(chan singleflight),
		done:        make(chan struct{}),
		err:         smithysync.NewOnceErr(),
	}

	go w.writeStream()

	return w
}

func (w *EventStreamWriter) writeStream() {
	defer w.Close()

	for {
		select {
		case ev := <-w.stream:
			err := w.protocol.SerializeEventMessage(w.schema, ev.variant, ev.event, w.eventStream)
			if err != nil {
				w.err.SetError(err)
			}
			ev.errCh <- err
		case <-w.done:
			return
		}
	}
}

// Send writes a single event to the stream.
func (w *EventStreamWriter) Send(ctx context.Context, variant *smithy.Schema, event smithy.Serializable) error {
	if err := w.err.Err(); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	select {
	case w.stream <- singleflight{variant, event, errCh}:
	case <-ctx.Done():
		return ctx.Err()
	case <-w.done:
		return fmt.Errorf("stream closed, unable to send event")
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-w.done:
		return fmt.Errorf("stream closed, unable to send event")
	}
}

// Close signals end-of-stream and closes the underlying writer. Close is
// safe for concurrent calls.
func (w *EventStreamWriter) Close() error {
	w.closeOnce.Do(func() {
		close(w.done)
		w.err.SetError(w.eventStream.Close())
	})
	return w.err.Err()
}

// Err returns the first error encountered during writing.
func (w *EventStreamWriter) Err() error {
	return w.err.Err()
}

// ErrorSet returns a channel that is closed when an error occurs.
func (w *EventStreamWriter) ErrorSet() <-chan struct{} {
	return w.err.ErrorSet()
}

// EventStreamReader reads events from a stream using a ClientProtocol.
type EventStreamReader struct {
	protocol ClientProtocol
	schema   *smithy.Schema
	types    *smithy.TypeRegistry

	eventStream io.ReadCloser
	stream      chan smithy.Deserializable
	done        chan struct{}
	err         *smithysync.OnceErr

	closeOnce sync.Once
}

// NewEventStreamReader returns an EventStreamReader that deserializes events
// through the given protocol from r. The schema is the event stream union
// schema.
func NewEventStreamReader(protocol ClientProtocol, schema *smithy.Schema, types *smithy.TypeRegistry, stream io.ReadCloser) *EventStreamReader {
	r := &EventStreamReader{
		protocol: protocol,
		schema:   schema,
		types:    types,

		eventStream: stream,
		stream:      make(chan smithy.Deserializable),
		done:        make(chan struct{}),
		err:         smithysync.NewOnceErr(),
	}

	go r.readEventStream()

	return r
}

func (r *EventStreamReader) readEventStream() {
	defer r.Close()
	defer close(r.stream)

	for {
		event, err := r.protocol.DeserializeEventMessage(r.schema, r.types, r.eventStream)
		if err != nil {
			if err == io.EOF {
				return
			}
			select {
			case <-r.done:
				return
			default:
				r.err.SetError(err)
				return
			}
		}

		select {
		case r.stream <- event:
		case <-r.done:
			return
		}
	}
}

// Events returns the channel from which deserialized events can be read.
func (r *EventStreamReader) Events() <-chan smithy.Deserializable {
	return r.stream
}

// Close stops the reader and releases the underlying stream. Close is safe
// for concurrent calls.
func (r *EventStreamReader) Close() error {
	r.closeOnce.Do(func() {
		close(r.done)
		r.eventStream.Close()
	})
	return r.err.Err()
}

// Err returns the first error encountered during reading.
func (r *EventStreamReader) Err() error {
	return r.err.Err()
}

// ErrorSet returns a channel that is closed when an error occurs.
func (r *EventStreamReader) ErrorSet() <-chan struct{} {
	return r.err.ErrorSet()
}

// Closed returns a channel that is closed when the reader is closed.
func (r *EventStreamReader) Closed() <-chan struct{} {
	return r.done
}
