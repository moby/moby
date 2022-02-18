package logger // import "github.com/moby/moby/daemon/logger"

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moby/moby/api/types/plugins/logdriver"
	"github.com/moby/moby/pkg/plugingetter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// pluginAdapter takes a plugin and implements the Logger interface for logger
// instances
type pluginAdapter struct {
	driverName   string
	id           string
	plugin       logPlugin
	fifoPath     string
	capabilities Capability
	logInfo      Info

	// synchronize access to the log stream and shared buffer
	mu     sync.Mutex
	enc    logdriver.LogEntryEncoder
	stream io.WriteCloser
	// buf is shared for each `Log()` call to reduce allocations.
	// buf must be protected by mutex
	buf logdriver.LogEntry
}

func (a *pluginAdapter) Log(msg *Message) error {
	a.mu.Lock()

	a.buf.Line = msg.Line
	a.buf.TimeNano = msg.Timestamp.UnixNano()
	a.buf.Partial = msg.PLogMetaData != nil
	a.buf.Source = msg.Source
	if msg.PLogMetaData != nil {
		a.buf.PartialLogMetadata = &logdriver.PartialLogEntryMetadata{
			Id:      msg.PLogMetaData.ID,
			Last:    msg.PLogMetaData.Last,
			Ordinal: int32(msg.PLogMetaData.Ordinal),
		}
	}

	err := a.enc.Encode(&a.buf)
	a.buf.Reset()

	a.mu.Unlock()

	PutMessage(msg)
	return err
}

func (a *pluginAdapter) Name() string {
	return a.driverName
}

func (a *pluginAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.plugin.StopLogging(filepath.Join("/", "run", "docker", "logging", a.id)); err != nil {
		return err
	}

	if err := a.stream.Close(); err != nil {
		logrus.WithError(err).Error("error closing plugin fifo")
	}
	if err := os.Remove(a.fifoPath); err != nil && !os.IsNotExist(err) {
		logrus.WithError(err).Error("error cleaning up plugin fifo")
	}

	// may be nil, especially for unit tests
	if pluginGetter != nil {
		pluginGetter.Get(a.Name(), extName, plugingetter.Release)
	}
	return nil
}

type pluginAdapterWithRead struct {
	*pluginAdapter
}

func (a *pluginAdapterWithRead) ReadLogs(config ReadConfig) *LogWatcher {
	watcher := NewLogWatcher()

	go func() {
		defer close(watcher.Msg)
		stream, err := a.plugin.ReadLogs(a.logInfo, config)
		if err != nil {
			watcher.Err <- errors.Wrap(err, "error getting log reader")
			return
		}
		defer stream.Close()

		dec := logdriver.NewLogEntryDecoder(stream)
		for {
			var buf logdriver.LogEntry
			if err := dec.Decode(&buf); err != nil {
				if err == io.EOF {
					return
				}
				watcher.Err <- errors.Wrap(err, "error decoding log message")
				return
			}

			msg := &Message{
				Timestamp: time.Unix(0, buf.TimeNano),
				Line:      buf.Line,
				Source:    buf.Source,
			}

			// plugin should handle this, but check just in case
			if !config.Since.IsZero() && msg.Timestamp.Before(config.Since) {
				continue
			}
			if !config.Until.IsZero() && msg.Timestamp.After(config.Until) {
				return
			}

			// send the message unless the consumer is gone
			select {
			case watcher.Msg <- msg:
			case <-watcher.WatchConsumerGone():
				return
			}
		}
	}()

	return watcher
}
