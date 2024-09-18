package cache

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type fakeLogger struct {
	messages chan logger.Message
	close    chan struct{}
}

func (l *fakeLogger) Log(msg *logger.Message) error {
	select {
	case l.messages <- *msg:
	case <-l.close:
	}
	logger.PutMessage(msg)
	return nil
}

func (l *fakeLogger) Name() string {
	return "fake"
}

func (l *fakeLogger) Close() error {
	close(l.close)
	return nil
}

func TestLog(t *testing.T) {
	cacher := &fakeLogger{make(chan logger.Message), make(chan struct{})}
	l := &loggerWithCache{
		l:     &fakeLogger{make(chan logger.Message, 100), make(chan struct{})},
		cache: cacher,
	}
	defer l.Close()

	var messages []logger.Message
	for i := 0; i < 100; i++ {
		messages = append(messages, logger.Message{
			Timestamp: time.Now(),
			Line:      append(bytes.Repeat([]byte("a"), 100), '\n'),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() {
		for _, msg := range messages {
			select {
			case <-ctx.Done():
				return
			default:
			}

			m := logger.NewMessage()
			dumbCopyMessage(m, &msg)
			l.Log(m)
		}
	}()

	for _, m := range messages {
		var msg logger.Message
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for messages... this is probably a test implementation error")
		case msg = <-cacher.messages:
			assert.Assert(t, is.DeepEqual(msg, m))
		}
	}
}
