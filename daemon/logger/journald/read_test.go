//go:build linux && cgo && !static_build && journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

import (
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"gotest.tools/v3/assert"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/journald/internal/fake"
	"github.com/docker/docker/daemon/logger/loggertest"
)

func TestLogRead(t *testing.T) {
	r := loggertest.Reader{
		Factory: func(t *testing.T, info logger.Info) func(*testing.T) logger.Logger {
			journalDir := t.TempDir()

			// Fill the journal with irrelevant events which the
			// LogReader needs to filter out.
			rotatedJournal := fake.NewT(t, journalDir+"/rotated.journal")
			rotatedJournal.AssignEventTimestampFromSyslogTimestamp = true
			l, err := new(logger.Info{
				ContainerID:   "wrongone0001",
				ContainerName: "fake",
			})
			assert.NilError(t, err)
			l.sendToJournal = rotatedJournal.Send
			assert.NilError(t, l.Log(&logger.Message{Source: "stdout", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("stdout of a different container in a rotated journal file")}))
			assert.NilError(t, l.Log(&logger.Message{Source: "stderr", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("stderr of a different container in a rotated journal file")}))
			assert.NilError(t, rotatedJournal.Send("a log message from a totally different process in a rotated journal", journal.PriInfo, nil))

			activeJournal := fake.NewT(t, journalDir+"/fake.journal")
			activeJournal.AssignEventTimestampFromSyslogTimestamp = true
			l, err = new(logger.Info{
				ContainerID:   "wrongone0002",
				ContainerName: "fake",
			})
			assert.NilError(t, err)
			l.sendToJournal = activeJournal.Send
			assert.NilError(t, l.Log(&logger.Message{Source: "stdout", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("stdout of a different container in the active journal file")}))
			assert.NilError(t, l.Log(&logger.Message{Source: "stderr", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("stderr of a different container in the active journal file")}))
			assert.NilError(t, rotatedJournal.Send("a log message from a totally different process in the active journal", journal.PriInfo, nil))

			return func(t *testing.T) logger.Logger {
				l, err := new(info)
				assert.NilError(t, err)
				l.journalReadDir = journalDir
				sl := &syncLogger{journald: l, waiters: map[uint64]chan<- struct{}{}}

				s := make(chan sendit, 100)
				t.Cleanup(func() { close(s) })
				go func() {
					for m := range s {
						<-m.after
						activeJournal.Send(m.message, m.priority, m.vars)
						sl.mu.Lock()
						sl.sent++
						if notify, ok := sl.waiters[sl.sent]; ok {
							delete(sl.waiters, sl.sent)
							close(notify)
						}
						sl.mu.Unlock()
					}
				}()

				l.sendToJournal = func(message string, priority journal.Priority, vars map[string]string) error {
					sl.mu.Lock()
					sl.queued++
					sl.mu.Unlock()
					s <- sendit{
						message:  message,
						priority: priority,
						vars:     vars,
						after:    time.After(150 * time.Millisecond),
					}
					return nil
				}
				l.readSyncTimeout = 3 * time.Second
				return sl
			}
		},
	}
	t.Run("Tail", r.TestTail)
	t.Run("Follow", r.TestFollow)
}

type sendit struct {
	message  string
	priority journal.Priority
	vars     map[string]string
	after    <-chan time.Time
}

type syncLogger struct {
	*journald

	mu           sync.Mutex
	queued, sent uint64
	waiters      map[uint64]chan<- struct{}
}

func (l *syncLogger) Sync() error {
	l.mu.Lock()
	waitFor := l.queued
	if l.sent >= l.queued {
		l.mu.Unlock()
		return nil
	}
	notify := make(chan struct{})
	l.waiters[waitFor] = notify
	l.mu.Unlock()
	<-notify
	return nil
}

func (l *syncLogger) Close() error {
	_ = l.Sync()
	return l.journald.Close()
}
