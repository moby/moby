package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"io"
	"os"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/filenotify"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var errRetry = errors.New("retry")
var errDone = errors.New("done")

type follow struct {
	file                      *os.File
	dec                       Decoder
	fileWatcher               filenotify.FileWatcher
	logWatcher                *logger.LogWatcher
	producerGone              <-chan struct{}
	draining                  bool
	notifyRotate, notifyEvict chan interface{}
	oldSize                   int64
	retries                   int
}

func (fl *follow) handleRotate() error {
	name := fl.file.Name()

	fl.file.Close()
	fl.fileWatcher.Remove(name)

	// retry when the file doesn't exist
	var err error
	for retries := 0; retries <= 5; retries++ {
		f, err := open(name)
		if err == nil || !os.IsNotExist(err) {
			fl.file = f
			break
		}
	}
	if err != nil {
		return err
	}
	if err := fl.fileWatcher.Add(name); err != nil {
		return err
	}
	fl.dec.Reset(fl.file)
	return nil
}

func (fl *follow) handleMustClose(evictErr error) {
	fl.file.Close()
	fl.dec.Close()
	fl.logWatcher.Err <- errors.Wrap(evictErr, "log reader evicted due to errors")
	logrus.WithField("file", fl.file.Name()).Error("Log reader notified that it must re-open log file, some log data may not be streamed to the client.")
}

func (fl *follow) waitRead() error {
	select {
	case e := <-fl.notifyEvict:
		if e != nil {
			err := e.(error)
			fl.handleMustClose(err)
		}
		return errDone
	case e := <-fl.fileWatcher.Events():
		switch e.Op {
		case fsnotify.Write:
			fl.dec.Reset(fl.file)
			return nil
		case fsnotify.Rename, fsnotify.Remove:
			select {
			case <-fl.notifyRotate:
			case <-fl.producerGone:
				return errDone
			case <-fl.logWatcher.WatchConsumerGone():
				return errDone
			}
			if err := fl.handleRotate(); err != nil {
				return err
			}
			return nil
		}
		return errRetry
	case err := <-fl.fileWatcher.Errors():
		logrus.Debugf("logger got error watching file: %v", err)
		// Something happened, let's try and stay alive and create a new watcher
		if fl.retries <= 5 {
			fl.fileWatcher.Close()
			fl.fileWatcher, err = watchFile(fl.file.Name())
			if err != nil {
				return err
			}
			fl.retries++
			return errRetry
		}
		return err
	case <-fl.producerGone:
		// There may be messages written out which the fileWatcher has
		// not yet notified us about.
		if fl.draining {
			return errDone
		}
		fl.draining = true
		fl.dec.Reset(fl.file)
		return nil
	case <-fl.logWatcher.WatchConsumerGone():
		return errDone
	}
}

func (fl *follow) handleDecodeErr(err error) error {
	if !errors.Is(err, io.EOF) {
		return err
	}

	// Handle special case (#39235): max-file=1 and file was truncated
	st, stErr := fl.file.Stat()
	if stErr == nil {
		size := st.Size()
		defer func() { fl.oldSize = size }()
		if size < fl.oldSize { // truncated
			fl.file.Seek(0, 0)
			fl.dec.Reset(fl.file)
			return nil
		}
	} else {
		logrus.WithError(stErr).Warn("logger: stat error")
	}

	for {
		err := fl.waitRead()
		if err == nil {
			break
		}
		if err == errRetry {
			continue
		}
		return err
	}
	return nil
}

func (fl *follow) mainLoop(since, until time.Time) {
	for {
		select {
		case err := <-fl.notifyEvict:
			if err != nil {
				fl.handleMustClose(err.(error))
			}
			return
		default:
		}
		msg, err := fl.dec.Decode()
		if err != nil {
			if err := fl.handleDecodeErr(err); err != nil {
				if err == errDone {
					return
				}
				// we got an unrecoverable error, so return
				fl.logWatcher.Err <- err
				return
			}
			// ready to try again
			continue
		}

		fl.retries = 0 // reset retries since we've succeeded
		if !since.IsZero() && msg.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && msg.Timestamp.After(until) {
			return
		}
		// send the message, unless the consumer is gone
		select {
		case e := <-fl.notifyEvict:
			if e != nil {
				err := e.(error)
				logrus.WithError(err).Debug("Reader evicted while sending log message")
				fl.logWatcher.Err <- err
			}
			return
		case fl.logWatcher.Msg <- msg:
		case <-fl.logWatcher.WatchConsumerGone():
			return
		}
	}
}

func followLogs(f *os.File, logWatcher *logger.LogWatcher, producerGone <-chan struct{}, notifyRotate, notifyEvict chan interface{}, dec Decoder, since, until time.Time) {
	dec.Reset(f)

	name := f.Name()
	fileWatcher, err := watchFile(name)
	if err != nil {
		logWatcher.Err <- err
		return
	}
	defer func() {
		f.Close()
		dec.Close()
		fileWatcher.Close()
	}()

	fl := &follow{
		file:         f,
		oldSize:      -1,
		logWatcher:   logWatcher,
		fileWatcher:  fileWatcher,
		producerGone: producerGone,
		notifyRotate: notifyRotate,
		notifyEvict:  notifyEvict,
		dec:          dec,
	}
	fl.mainLoop(since, until)
}
