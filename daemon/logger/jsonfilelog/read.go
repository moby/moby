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
	"github.com/docker/docker/pkg/filenotify"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/tailfile"
)

const maxJSONDecodeRetry = 20000

func decodeLogLine(dec *json.Decoder, l *jsonlog.JSONLog) (*logger.Message, error) {
	l.Reset()
	if err := dec.Decode(l); err != nil {
		return nil, err
	}
	msg := &logger.Message{
		Source:    l.Stream,
		Timestamp: l.Created,
		Line:      []byte(l.Log),
		Attrs:     l.Attrs,
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
		files = append(files, f)
	}

	latestFile, err := os.Open(pth)
	if err != nil {
		logWatcher.Err <- err
		return
	}

	if config.Tail != 0 {
		tailer := ioutils.MultiReadSeeker(append(files, latestFile)...)
		tailFile(tailer, logWatcher, config.Tail, config.Since)
	}

	// close all the rotated files
	for _, f := range files {
		if err := f.(io.Closer).Close(); err != nil {
			logrus.WithField("logger", "json-file").Warnf("error closing tailed log file: %v", err)
		}
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
	followLogs(latestFile, logWatcher, notifyRotate, config.Since)

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

func followLogs(f *os.File, logWatcher *logger.LogWatcher, notifyRotate chan interface{}, since time.Time) {
	dec := json.NewDecoder(f)
	l := &jsonlog.JSONLog{}

	fileWatcher, err := filenotify.New()
	if err != nil {
		logWatcher.Err <- err
	}
	defer func() {
		f.Close()
		fileWatcher.Close()
	}()
	name := f.Name()

	if err := fileWatcher.Add(name); err != nil {
		logrus.WithField("logger", "json-file").Warnf("falling back to file poller due to error: %v", err)
		fileWatcher.Close()
		fileWatcher = filenotify.NewPollingWatcher()

		if err := fileWatcher.Add(name); err != nil {
			logrus.Debugf("error watching log file for modifications: %v", err)
			logWatcher.Err <- err
			return
		}
	}

	var retries int
	for {
		msg, err := decodeLogLine(dec, l)
		if err != nil {
			if err != io.EOF {
				// try again because this shouldn't happen
				if _, ok := err.(*json.SyntaxError); ok && retries <= maxJSONDecodeRetry {
					dec = json.NewDecoder(f)
					retries++
					continue
				}

				// io.ErrUnexpectedEOF is returned from json.Decoder when there is
				// remaining data in the parser's buffer while an io.EOF occurs.
				// If the json logger writes a partial json log entry to the disk
				// while at the same time the decoder tries to decode it, the race condition happens.
				if err == io.ErrUnexpectedEOF && retries <= maxJSONDecodeRetry {
					reader := io.MultiReader(dec.Buffered(), f)
					dec = json.NewDecoder(reader)
					retries++
					continue
				}

				return
			}

			select {
			case <-fileWatcher.Events():
				dec = json.NewDecoder(f)
				continue
			case <-fileWatcher.Errors():
				logWatcher.Err <- err
				return
			case <-logWatcher.WatchClose():
				fileWatcher.Remove(name)
				return
			case <-notifyRotate:
				f.Close()
				fileWatcher.Remove(name)

				// retry when the file doesn't exist
				for retries := 0; retries <= 5; retries++ {
					f, err = os.Open(name)
					if err == nil || !os.IsNotExist(err) {
						break
					}
				}

				if err = fileWatcher.Add(name); err != nil {
					logWatcher.Err <- err
					return
				}
				if err != nil {
					logWatcher.Err <- err
					return
				}

				dec = json.NewDecoder(f)
				continue
			}
		}

		retries = 0 // reset retries since we've succeeded
		if !since.IsZero() && msg.Timestamp.Before(since) {
			continue
		}
		select {
		case logWatcher.Msg <- msg:
		case <-logWatcher.WatchClose():
			logWatcher.Msg <- msg
			for {
				msg, err := decodeLogLine(dec, l)
				if err != nil {
					return
				}
				if !since.IsZero() && msg.Timestamp.Before(since) {
					continue
				}
				logWatcher.Msg <- msg
			}
		}
	}
}
