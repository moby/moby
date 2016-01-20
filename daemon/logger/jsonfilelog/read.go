package jsonfilelog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/tailfile"
)

func decodeLogLine(dec *json.Decoder, l *jsonlog.JSONLog) (*logger.Message, error) {
	l.Reset()
	if err := dec.Decode(l); err != nil {
		return nil, err
	}
	msg := &logger.Message{
		Source:    l.Stream,
		Timestamp: l.Created,
		Line:      []byte(l.Log),
	}
	return msg, nil
}

// ReadLogs implements the logger's LogReader interface for the logs
// created by this driver.
func (l *JSONFileLogger) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()
	go l.readLogs(logWatcher, config)
	return logWatcher
}

func (l *JSONFileLogger) readLogs(logWatcher *logger.LogWatcher, config logger.ReadConfig) {
	defer close(logWatcher.Msg)

	pth := l.writer.LogPath()
	var files []io.ReadSeeker
	for i := l.writer.MaxFiles(); i > 1; i-- {
		f, err := os.Open(fmt.Sprintf("%s.%d", pth, i-1))
		if err != nil {
			if !os.IsNotExist(err) {
				logWatcher.Err <- err
				break
			}
			continue
		}
		defer f.Close()
		files = append(files, f)
	}

	latestFile, err := os.Open(pth)
	if err != nil {
		logWatcher.Err <- err
		return
	}
	defer latestFile.Close()

	files = append(files, latestFile)
	tailer := ioutils.MultiReadSeeker(files...)

	if config.Tail != 0 {
		tailFile(tailer, logWatcher, config.Tail, config.Since)
	}

	if !config.Follow {
		return
	}

	if config.Tail >= 0 {
		latestFile.Seek(0, os.SEEK_END)
	}

	l.mu.Lock()
	l.readers[logWatcher] = struct{}{}
	l.mu.Unlock()

	notifyRotate := l.writer.NotifyRotate()
	l.followLogs(latestFile, logWatcher, notifyRotate, config.Since)

	l.mu.Lock()
	delete(l.readers, logWatcher)
	l.mu.Unlock()

	l.writer.NotifyRotateEvict(notifyRotate)
}

func tailFile(f io.ReadSeeker, logWatcher *logger.LogWatcher, tail int, since time.Time) {
	var rdr io.Reader = f
	if tail > 0 {
		ls, err := tailfile.TailFile(f, tail)
		if err != nil {
			logWatcher.Err <- err
			return
		}
		rdr = bytes.NewBuffer(bytes.Join(ls, []byte("\n")))
	}
	dec := json.NewDecoder(rdr)
	l := &jsonlog.JSONLog{}
	for {
		msg, err := decodeLogLine(dec, l)
		if err != nil {
			if err != io.EOF {
				logWatcher.Err <- err
			}
			return
		}
		if !since.IsZero() && msg.Timestamp.Before(since) {
			continue
		}
		logWatcher.Msg <- msg
	}
}

func (l *JSONFileLogger) followLogs(f *os.File, logWatcher *logger.LogWatcher, notifyRotate chan interface{}, since time.Time) {
	var (
		rotated bool

		dec         = json.NewDecoder(f)
		log         = &jsonlog.JSONLog{}
		writeNotify = l.writeNotifier.Subscribe()
		watchClose  = logWatcher.WatchClose()
	)

	reopenLogFile := func() error {
		f.Close()
		f, err := os.Open(f.Name())
		if err != nil {
			return err
		}
		dec = json.NewDecoder(f)
		rotated = true
		return nil
	}

	readToEnd := func() error {
		for {
			msg, err := decodeLogLine(dec, log)
			if err != nil {
				return err
			}
			if !since.IsZero() && msg.Timestamp.Before(since) {
				continue
			}
			logWatcher.Msg <- msg
		}
	}

	defer func() {
		l.writeNotifier.Evict(writeNotify)
		if rotated {
			f.Close()
		}
	}()

	for {
		select {
		case <-watchClose:
			readToEnd()
			return
		case <-notifyRotate:
			readToEnd()
			if err := reopenLogFile(); err != nil {
				logWatcher.Err <- err
				return
			}
		case _, ok := <-writeNotify:
			if err := readToEnd(); err == io.EOF {
				if !ok {
					// The writer is closed, no new logs will be generated.
					return
				}

				select {
				case <-notifyRotate:
					if err := reopenLogFile(); err != nil {
						logWatcher.Err <- err
						return
					}
				default:
					dec = json.NewDecoder(f)
				}

			} else if err == io.ErrUnexpectedEOF {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), f))
			} else {
				logrus.Errorf("Failed to decode json log %s: %v", f.Name(), err)
				logWatcher.Err <- err
				return
			}
		}
	}
}
