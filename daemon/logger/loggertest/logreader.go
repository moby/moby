package loggertest // import "github.com/docker/docker/daemon/logger/loggertest"

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/opt"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
)

// Reader tests that a logger.LogReader implementation behaves as it should.
type Reader struct {
	// Factory returns a function which constructs loggers for the container
	// specified in info. Each call to the returned function must yield a
	// distinct logger instance which can read back logs written by earlier
	// instances.
	Factory func(*testing.T, logger.Info) func(*testing.T) logger.Logger
}

var compareLog cmp.Options = []cmp.Option{
	// Not all log drivers can round-trip timestamps at full nanosecond
	// precision.
	opt.TimeWithThreshold(time.Millisecond),
	// The json-log driver does not round-trip PLogMetaData and API users do
	// not expect it.
	cmpopts.IgnoreFields(logger.Message{}, "PLogMetaData"),
	cmp.Transformer("string", func(b []byte) string { return string(b) }),
}

// TestTail tests the behavior of the LogReader's tail implementation.
func (tr Reader) TestTail(t *testing.T) {
	t.Run("Live", func(t *testing.T) { tr.testTail(t, true) })
	t.Run("LiveEmpty", func(t *testing.T) { tr.testTailEmptyLogs(t, true) })
	t.Run("Stopped", func(t *testing.T) { tr.testTail(t, false) })
	t.Run("StoppedEmpty", func(t *testing.T) { tr.testTailEmptyLogs(t, false) })
}

func makeTestMessages() []*logger.Message {
	return []*logger.Message{
		{Source: "stdout", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("a message")},
		{Source: "stdout", Timestamp: time.Now().Add(-1 * 20 * time.Minute), Line: []byte("another message"), PLogMetaData: &backend.PartialLogMetaData{ID: "aaaaaaaa", Ordinal: 1, Last: true}},
		{Source: "stderr", Timestamp: time.Now().Add(-1 * 15 * time.Minute), Line: []byte("to be..."), PLogMetaData: &backend.PartialLogMetaData{ID: "bbbbbbbb", Ordinal: 1}},
		{Source: "stderr", Timestamp: time.Now().Add(-1 * 15 * time.Minute), Line: []byte("continued"), PLogMetaData: &backend.PartialLogMetaData{ID: "bbbbbbbb", Ordinal: 2, Last: true}},
		{Source: "stderr", Timestamp: time.Now().Add(-1 * 10 * time.Minute), Line: []byte("a really long message " + strings.Repeat("a", 4096))},
		{Source: "stderr", Timestamp: time.Now().Add(-1 * 10 * time.Minute), Line: []byte("just one more message")},
		{Source: "stdout", Timestamp: time.Now().Add(-1 * 90 * time.Minute), Line: []byte("someone adjusted the clock")},
	}

}

func (tr Reader) testTail(t *testing.T, live bool) {
	t.Parallel()
	factory := tr.Factory(t, logger.Info{
		ContainerID:   "tailtest0000",
		ContainerName: "logtail",
	})
	l := factory(t)
	if live {
		defer func() { assert.NilError(t, l.Close()) }()
	}

	mm := makeTestMessages()
	expected := logMessages(t, l, mm)

	if !live {
		// Simulate reading from a stopped container.
		assert.NilError(t, l.Close())
		l = factory(t)
		defer func() { assert.NilError(t, l.Close()) }()
	}
	lr := l.(logger.LogReader)

	t.Run("Exact", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: len(mm)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected, compareLog)
	})

	t.Run("LessThanAvailable", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: 2})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[len(mm)-2:], compareLog)
	})

	t.Run("MoreThanAvailable", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: 100})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected, compareLog)
	})

	t.Run("All", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: -1})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected, compareLog)
	})

	t.Run("Since", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: -1, Since: mm[1].Timestamp.Truncate(time.Millisecond)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[1:], compareLog)
	})

	t.Run("MoreThanSince", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: len(mm), Since: mm[1].Timestamp.Truncate(time.Millisecond)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[1:], compareLog)
	})

	t.Run("LessThanSince", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: len(mm) - 2, Since: mm[1].Timestamp.Truncate(time.Millisecond)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[2:], compareLog)
	})

	t.Run("Until", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: -1, Until: mm[2].Timestamp.Add(-time.Millisecond)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[:2], compareLog)
	})

	t.Run("SinceAndUntil", func(t *testing.T) {
		t.Parallel()
		lw := lr.ReadLogs(logger.ReadConfig{Tail: -1, Since: mm[1].Timestamp.Truncate(time.Millisecond), Until: mm[1].Timestamp.Add(time.Millisecond)})
		defer lw.ConsumerGone()
		assert.DeepEqual(t, readAll(t, lw), expected[1:2], compareLog)
	})
}

func (tr Reader) testTailEmptyLogs(t *testing.T, live bool) {
	t.Parallel()
	factory := tr.Factory(t, logger.Info{
		ContainerID:   "tailemptytest",
		ContainerName: "logtail",
	})
	l := factory(t)
	if !live {
		assert.NilError(t, l.Close())
		l = factory(t)
	}
	defer func() { assert.NilError(t, l.Close()) }()

	for _, tt := range []struct {
		name string
		cfg  logger.ReadConfig
	}{
		{name: "Zero", cfg: logger.ReadConfig{}},
		{name: "All", cfg: logger.ReadConfig{Tail: -1}},
		{name: "Tail", cfg: logger.ReadConfig{Tail: 42}},
		{name: "Since", cfg: logger.ReadConfig{Since: time.Unix(1, 0)}},
		{name: "Until", cfg: logger.ReadConfig{Until: time.Date(2100, time.January, 1, 1, 1, 1, 0, time.UTC)}},
		{name: "SinceAndUntil", cfg: logger.ReadConfig{Since: time.Unix(1, 0), Until: time.Date(2100, time.January, 1, 1, 1, 1, 0, time.UTC)}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{})
			defer lw.ConsumerGone()
			assert.DeepEqual(t, readAll(t, lw), ([]*logger.Message)(nil), cmpopts.EquateEmpty())
		})
	}
}

// TestFollow tests the LogReader's follow implementation.
//
// The LogReader is expected to be able to follow an arbitrary number of
// messages at a high rate with no dropped messages.
func (tr Reader) TestFollow(t *testing.T) {
	// Reader sends all logs and closes after logger is closed
	// - Starting from empty log (like run)
	t.Run("FromEmptyLog", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followstart0",
			ContainerName: "logloglog",
		})(t)
		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		mm := makeTestMessages()
		expected := logMessages(t, l, mm)
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("AttachMidStream", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followmiddle",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()
		expected := logMessages(t, l, mm[0:1])

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected = append(expected, logMessages(t, l, mm[1:])...)
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("Since", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followsince0",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true, Since: mm[2].Timestamp.Truncate(time.Millisecond)})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected := logMessages(t, l, mm)[2:]
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("Until", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followuntil0",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true, Until: mm[2].Timestamp.Add(-time.Millisecond)})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected := logMessages(t, l, mm)[:2]
		defer assert.NilError(t, l.Close()) // Reading should end before the logger is closed.
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("SinceAndUntil", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followbounded",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true, Since: mm[1].Timestamp.Add(-time.Millisecond), Until: mm[2].Timestamp.Add(-time.Millisecond)})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected := logMessages(t, l, mm)[1:2]
		defer assert.NilError(t, l.Close()) // Reading should end before the logger is closed.
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("Tail=0", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followtail00",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()
		logMessages(t, l, mm[0:2])

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: 0, Follow: true})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected := logMessages(t, l, mm[2:])
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("Tail>0", func(t *testing.T) {
		t.Parallel()
		l := tr.Factory(t, logger.Info{
			ContainerID:   "followtail00",
			ContainerName: "logloglog",
		})(t)

		mm := makeTestMessages()
		expected := logMessages(t, l, mm[0:2])[1:]

		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: 1, Follow: true})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected = append(expected, logMessages(t, l, mm[2:])...)
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("MultipleStarts", func(t *testing.T) {
		t.Parallel()
		factory := tr.Factory(t, logger.Info{
			ContainerID:   "startrestart",
			ContainerName: "startmeup",
		})

		mm := makeTestMessages()
		l := factory(t)
		expected := logMessages(t, l, mm[:3])
		assert.NilError(t, l.Close())

		l = factory(t)
		lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Tail: -1, Follow: true})
		defer lw.ConsumerGone()

		doneReading := make(chan struct{})
		var logs []*logger.Message
		go func() {
			defer close(doneReading)
			logs = readAll(t, lw)
		}()

		expected = append(expected, logMessages(t, l, mm[3:])...)
		assert.NilError(t, l.Close())
		<-doneReading
		assert.DeepEqual(t, logs, expected, compareLog)
	})

	t.Run("Concurrent", tr.TestConcurrent)
}

// TestConcurrent tests the Logger and its LogReader implementation for
// race conditions when logging from multiple goroutines concurrently.
func (tr Reader) TestConcurrent(t *testing.T) {
	t.Parallel()
	l := tr.Factory(t, logger.Info{
		ContainerID:   "logconcurrent0",
		ContainerName: "logconcurrent123",
	})(t)

	// Split test messages
	stderrMessages := []*logger.Message{}
	stdoutMessages := []*logger.Message{}
	for _, m := range makeTestMessages() {
		if m.Source == "stdout" {
			stdoutMessages = append(stdoutMessages, m)
		} else if m.Source == "stderr" {
			stderrMessages = append(stderrMessages, m)
		}
	}

	// Follow all logs
	lw := l.(logger.LogReader).ReadLogs(logger.ReadConfig{Follow: true, Tail: -1})
	defer lw.ConsumerGone()

	// Log concurrently from two sources and close log
	wg := &sync.WaitGroup{}
	logAll := func(msgs []*logger.Message) {
		defer wg.Done()
		for _, m := range msgs {
			l.Log(copyLogMessage(m))
		}
	}

	closed := make(chan struct{})
	wg.Add(2)
	go logAll(stdoutMessages)
	go logAll(stderrMessages)
	go func() {
		defer close(closed)
		defer l.Close()
		wg.Wait()
	}()

	// Check if the message count, order and content is equal to what was logged
	for {
		l := readMessage(t, lw)
		if l == nil {
			break
		}

		var messages *[]*logger.Message
		if l.Source == "stdout" {
			messages = &stdoutMessages
		} else if l.Source == "stderr" {
			messages = &stderrMessages
		} else {
			t.Fatalf("Corrupted message.Source = %q", l.Source)
		}

		expectedMsg := transformToExpected((*messages)[0])

		assert.DeepEqual(t, *expectedMsg, *l, compareLog)
		*messages = (*messages)[1:]
	}

	assert.Equal(t, len(stdoutMessages), 0)
	assert.Equal(t, len(stderrMessages), 0)

	// Make sure log gets closed before we return
	// so the temporary dir can be deleted
	<-closed
}

// logMessages logs messages to l and returns a slice of messages as would be
// expected to be read back. The message values are not modified and the
// returned slice of messages are deep-copied.
func logMessages(t *testing.T, l logger.Logger, messages []*logger.Message) []*logger.Message {
	t.Helper()
	var expected []*logger.Message
	for _, m := range messages {
		// Copy the log message because the underlying log writer resets
		// the log message and returns it to a buffer pool.
		assert.NilError(t, l.Log(copyLogMessage(m)))
		runtime.Gosched()

		expect := transformToExpected(m)
		expected = append(expected, expect)
	}
	return expected
}

// Existing API consumers expect a newline to be appended to
// messages other than nonterminal partials as that matches the
// existing behavior of the json-file log driver.
func transformToExpected(m *logger.Message) *logger.Message {
	// Copy the log message again so as not to mutate the input.
	copy := copyLogMessage(m)
	if m.PLogMetaData == nil || m.PLogMetaData.Last {
		copy.Line = append(copy.Line, '\n')
	}

	return copy
}

func copyLogMessage(src *logger.Message) *logger.Message {
	dst := logger.NewMessage()
	dst.Source = src.Source
	dst.Timestamp = src.Timestamp
	dst.Attrs = src.Attrs
	dst.Err = src.Err
	dst.Line = append(dst.Line, src.Line...)
	if src.PLogMetaData != nil {
		lmd := *src.PLogMetaData
		dst.PLogMetaData = &lmd
	}
	return dst
}
func readMessage(t *testing.T, lw *logger.LogWatcher) *logger.Message {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	select {
	case <-timeout.C:
		t.Error("timed out waiting for message")
		return nil
	case err, open := <-lw.Err:
		t.Errorf("unexpected receive on lw.Err: err=%v, open=%v", err, open)
		return nil
	case msg, open := <-lw.Msg:
		if !open {
			select {
			case err, open := <-lw.Err:
				t.Errorf("unexpected receive on lw.Err with closed lw.Msg: err=%v, open=%v", err, open)
			default:
			}
			return nil
		}
		assert.Assert(t, msg != nil)
		t.Logf("[%v] %s: %s", msg.Timestamp, msg.Source, msg.Line)
		return msg
	}
}

func readAll(t *testing.T, lw *logger.LogWatcher) []*logger.Message {
	t.Helper()
	var msgs []*logger.Message
	for {
		m := readMessage(t, lw)
		if m == nil {
			return msgs
		}
		msgs = append(msgs, m)
	}
}
