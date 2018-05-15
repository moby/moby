package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils/multireader"
	"github.com/docker/docker/pkg/filenotify"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/pubsub"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const tmpLogfileSuffix = ".tmp"

// rotateFileMetadata is a metadata of the gzip header of the compressed log file
type rotateFileMetadata struct {
	LastTime time.Time `json:"lastTime,omitempty"`
}

// refCounter is a counter of logfile being referenced
type refCounter struct {
	mu      sync.Mutex
	counter map[string]int
}

// Reference increase the reference counter for specified logfile
func (rc *refCounter) GetReference(fileName string, openRefFile func(fileName string, exists bool) (*os.File, error)) (*os.File, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	var (
		file *os.File
		err  error
	)
	_, ok := rc.counter[fileName]
	file, err = openRefFile(fileName, ok)
	if err != nil {
		return nil, err
	}

	if ok {
		rc.counter[fileName]++
	} else if file != nil {
		rc.counter[file.Name()] = 1
	}

	return file, nil
}

// Dereference reduce the reference counter for specified logfile
func (rc *refCounter) Dereference(fileName string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.counter[fileName]--
	if rc.counter[fileName] <= 0 {
		delete(rc.counter, fileName)
		err := os.Remove(fileName)
		if err != nil {
			return err
		}
	}
	return nil
}

// LogFile is Logger implementation for default Docker logging.
type LogFile struct {
	mu              sync.RWMutex // protects the logfile access
	f               *os.File     // store for closing
	closed          bool
	rotateMu        sync.Mutex // blocks the next rotation until the current rotation is completed
	capacity        int64      // maximum size of each file
	currentSize     int64      // current size of the latest file
	maxFiles        int        // maximum number of files
	compress        bool       // whether old versions of log files are compressed
	lastTimestamp   time.Time  // timestamp of the last log
	filesRefCounter refCounter // keep reference-counted of decompressed files
	notifyRotate    *pubsub.Publisher
	marshal         logger.MarshalFunc
	createDecoder   makeDecoderFunc
	perms           os.FileMode
}

type makeDecoderFunc func(rdr io.Reader) func() (*logger.Message, error)

// NewLogFile creates new LogFile
func NewLogFile(logPath string, capacity int64, maxFiles int, compress bool, marshaller logger.MarshalFunc, decodeFunc makeDecoderFunc, perms os.FileMode) (*LogFile, error) {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, perms)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		f:               log,
		capacity:        capacity,
		currentSize:     size,
		maxFiles:        maxFiles,
		compress:        compress,
		filesRefCounter: refCounter{counter: make(map[string]int)},
		notifyRotate:    pubsub.NewPublisher(0, 1),
		marshal:         marshaller,
		createDecoder:   decodeFunc,
		perms:           perms,
	}, nil
}

// WriteLogEntry writes the provided log message to the current log file.
// This may trigger a rotation event if the max file/capacity limits are hit.
func (w *LogFile) WriteLogEntry(msg *logger.Message) error {
	b, err := w.marshal(msg)
	if err != nil {
		return errors.Wrap(err, "error marshalling log message")
	}

	logger.PutMessage(msg)

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("cannot write because the output file was closed")
	}

	if err := w.checkCapacityAndRotate(); err != nil {
		w.mu.Unlock()
		return err
	}

	n, err := w.f.Write(b)
	if err == nil {
		w.currentSize += int64(n)
		w.lastTimestamp = msg.Timestamp
	}
	w.mu.Unlock()
	return err
}

func (w *LogFile) checkCapacityAndRotate() error {
	if w.capacity == -1 {
		return nil
	}

	if w.currentSize >= w.capacity {
		w.rotateMu.Lock()
		fname := w.f.Name()
		if err := w.f.Close(); err != nil {
			w.rotateMu.Unlock()
			return errors.Wrap(err, "error closing file")
		}
		if err := rotate(fname, w.maxFiles, w.compress); err != nil {
			w.rotateMu.Unlock()
			return err
		}
		file, err := os.OpenFile(fname, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, w.perms)
		if err != nil {
			w.rotateMu.Unlock()
			return err
		}
		w.f = file
		w.currentSize = 0
		w.notifyRotate.Publish(struct{}{})

		if w.maxFiles <= 1 || !w.compress {
			w.rotateMu.Unlock()
			return nil
		}

		go func() {
			compressFile(fname+".1", w.lastTimestamp)
			w.rotateMu.Unlock()
		}()
	}

	return nil
}

func rotate(name string, maxFiles int, compress bool) error {
	if maxFiles < 2 {
		return nil
	}

	var extension string
	if compress {
		extension = ".gz"
	}

	lastFile := fmt.Sprintf("%s.%d%s", name, maxFiles-1, extension)
	err := os.Remove(lastFile)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "error removing oldest log file")
	}

	for i := maxFiles - 1; i > 1; i-- {
		toPath := name + "." + strconv.Itoa(i) + extension
		fromPath := name + "." + strconv.Itoa(i-1) + extension
		if err := os.Rename(fromPath, toPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := os.Rename(name, name+".1"); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func compressFile(fileName string, lastTimestamp time.Time) {
	file, err := os.Open(fileName)
	if err != nil {
		logrus.Errorf("Failed to open log file: %v", err)
		return
	}
	defer func() {
		file.Close()
		err := os.Remove(fileName)
		if err != nil {
			logrus.Errorf("Failed to remove source log file: %v", err)
		}
	}()

	outFile, err := os.OpenFile(fileName+".gz", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0640)
	if err != nil {
		logrus.Errorf("Failed to open or create gzip log file: %v", err)
		return
	}
	defer func() {
		outFile.Close()
		if err != nil {
			os.Remove(fileName + ".gz")
		}
	}()

	compressWriter := gzip.NewWriter(outFile)
	defer compressWriter.Close()

	// Add the last log entry timestramp to the gzip header
	extra := rotateFileMetadata{}
	extra.LastTime = lastTimestamp
	compressWriter.Header.Extra, err = json.Marshal(&extra)
	if err != nil {
		// Here log the error only and don't return since this is just an optimization.
		logrus.Warningf("Failed to marshal gzip header as JSON: %v", err)
	}

	_, err = pools.Copy(compressWriter, file)
	if err != nil {
		logrus.WithError(err).WithField("module", "container.logs").WithField("file", fileName).Error("Error compressing log file")
		return
	}
}

// MaxFiles return maximum number of files
func (w *LogFile) MaxFiles() int {
	return w.maxFiles
}

// Close closes underlying file and signals all readers to stop.
func (w *LogFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	w.closed = true
	return nil
}

// ReadLogs decodes entries from log files and sends them the passed in watcher
//
// Note: Using the follow option can become inconsistent in cases with very frequent rotations and max log files is 1.
// TODO: Consider a different implementation which can effectively follow logs under frequent rotations.
func (w *LogFile) ReadLogs(config logger.ReadConfig, watcher *logger.LogWatcher) {
	w.mu.RLock()
	currentFile, err := os.Open(w.f.Name())
	if err != nil {
		w.mu.RUnlock()
		watcher.Err <- err
		return
	}
	defer currentFile.Close()

	currentChunk, err := newSectionReader(currentFile)
	if err != nil {
		w.mu.RUnlock()
		watcher.Err <- err
		return
	}

	if config.Tail != 0 {
		files, err := w.openRotatedFiles(config)
		if err != nil {
			w.mu.RUnlock()
			watcher.Err <- err
			return
		}
		w.mu.RUnlock()
		seekers := make([]io.ReadSeeker, 0, len(files)+1)
		for _, f := range files {
			seekers = append(seekers, f)
		}
		if currentChunk.Size() > 0 {
			seekers = append(seekers, currentChunk)
		}
		if len(seekers) > 0 {
			tailFile(multireader.MultiReadSeeker(seekers...), watcher, w.createDecoder, config)
		}
		for _, f := range files {
			f.Close()
			fileName := f.Name()
			if strings.HasSuffix(fileName, tmpLogfileSuffix) {
				err := w.filesRefCounter.Dereference(fileName)
				if err != nil {
					logrus.Errorf("Failed to dereference the log file %q: %v", fileName, err)
				}
			}
		}

		w.mu.RLock()
	}

	if !config.Follow || w.closed {
		w.mu.RUnlock()
		return
	}
	w.mu.RUnlock()

	notifyRotate := w.notifyRotate.Subscribe()
	defer w.notifyRotate.Evict(notifyRotate)
	followLogs(currentFile, watcher, notifyRotate, w.createDecoder, config.Since, config.Until)
}

func (w *LogFile) openRotatedFiles(config logger.ReadConfig) (files []*os.File, err error) {
	w.rotateMu.Lock()
	defer w.rotateMu.Unlock()

	defer func() {
		if err == nil {
			return
		}
		for _, f := range files {
			f.Close()
			if strings.HasSuffix(f.Name(), tmpLogfileSuffix) {
				err := os.Remove(f.Name())
				if err != nil && !os.IsNotExist(err) {
					logrus.Warningf("Failed to remove the logfile %q: %v", f.Name, err)
				}
			}
		}
	}()

	for i := w.maxFiles; i > 1; i-- {
		f, err := os.Open(fmt.Sprintf("%s.%d", w.f.Name(), i-1))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, errors.Wrap(err, "error opening rotated log file")
			}

			fileName := fmt.Sprintf("%s.%d.gz", w.f.Name(), i-1)
			decompressedFileName := fileName + tmpLogfileSuffix
			tmpFile, err := w.filesRefCounter.GetReference(decompressedFileName, func(refFileName string, exists bool) (*os.File, error) {
				if exists {
					return os.Open(refFileName)
				}
				return decompressfile(fileName, refFileName, config.Since)
			})

			if err != nil {
				if !os.IsNotExist(errors.Cause(err)) {
					return nil, errors.Wrap(err, "error getting reference to decompressed log file")
				}
				continue
			}
			if tmpFile == nil {
				// The log before `config.Since` does not need to read
				break
			}

			files = append(files, tmpFile)
			continue
		}
		files = append(files, f)
	}

	return files, nil
}

func decompressfile(fileName, destFileName string, since time.Time) (*os.File, error) {
	cf, err := os.Open(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "error opening file for decompression")
	}
	defer cf.Close()

	rc, err := gzip.NewReader(cf)
	if err != nil {
		return nil, errors.Wrap(err, "error making gzip reader for compressed log file")
	}
	defer rc.Close()

	// Extract the last log entry timestramp from the gzip header
	extra := &rotateFileMetadata{}
	err = json.Unmarshal(rc.Header.Extra, extra)
	if err == nil && extra.LastTime.Before(since) {
		return nil, nil
	}

	rs, err := os.OpenFile(destFileName, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return nil, errors.Wrap(err, "error creating file for copying decompressed log stream")
	}

	_, err = pools.Copy(rs, rc)
	if err != nil {
		rs.Close()
		rErr := os.Remove(rs.Name())
		if rErr != nil && !os.IsNotExist(rErr) {
			logrus.Errorf("Failed to remove the logfile %q: %v", rs.Name(), rErr)
		}
		return nil, errors.Wrap(err, "error while copying decompressed log stream to file")
	}

	return rs, nil
}

func newSectionReader(f *os.File) (*io.SectionReader, error) {
	// seek to the end to get the size
	// we'll leave this at the end of the file since section reader does not advance the reader
	size, err := f.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, errors.Wrap(err, "error getting current file size")
	}
	return io.NewSectionReader(f, 0, size), nil
}

type decodeFunc func() (*logger.Message, error)

func tailFile(f io.ReadSeeker, watcher *logger.LogWatcher, createDecoder makeDecoderFunc, config logger.ReadConfig) {
	var rdr io.Reader = f
	if config.Tail > 0 {
		ls, err := tailfile.TailFile(f, config.Tail)
		if err != nil {
			watcher.Err <- err
			return
		}
		rdr = bytes.NewBuffer(bytes.Join(ls, []byte("\n")))
	}

	decodeLogLine := createDecoder(rdr)
	for {
		msg, err := decodeLogLine()
		if err != nil {
			if errors.Cause(err) != io.EOF {
				watcher.Err <- err
			}
			return
		}
		if !config.Since.IsZero() && msg.Timestamp.Before(config.Since) {
			continue
		}
		if !config.Until.IsZero() && msg.Timestamp.After(config.Until) {
			return
		}
		select {
		case <-watcher.WatchClose():
			return
		case watcher.Msg <- msg:
		}
	}
}

func followLogs(f *os.File, logWatcher *logger.LogWatcher, notifyRotate chan interface{}, createDecoder makeDecoderFunc, since, until time.Time) {
	decodeLogLine := createDecoder(f)

	name := f.Name()
	fileWatcher, err := watchFile(name)
	if err != nil {
		logWatcher.Err <- err
		return
	}
	defer func() {
		f.Close()
		fileWatcher.Remove(name)
		fileWatcher.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-logWatcher.WatchClose():
			fileWatcher.Remove(name)
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	var retries int
	handleRotate := func() error {
		f.Close()
		fileWatcher.Remove(name)

		// retry when the file doesn't exist
		for retries := 0; retries <= 5; retries++ {
			f, err = os.Open(name)
			if err == nil || !os.IsNotExist(err) {
				break
			}
		}
		if err != nil {
			return err
		}
		if err := fileWatcher.Add(name); err != nil {
			return err
		}
		decodeLogLine = createDecoder(f)
		return nil
	}

	errRetry := errors.New("retry")
	errDone := errors.New("done")
	waitRead := func() error {
		select {
		case e := <-fileWatcher.Events():
			switch e.Op {
			case fsnotify.Write:
				decodeLogLine = createDecoder(f)
				return nil
			case fsnotify.Rename, fsnotify.Remove:
				select {
				case <-notifyRotate:
				case <-ctx.Done():
					return errDone
				}
				if err := handleRotate(); err != nil {
					return err
				}
				return nil
			}
			return errRetry
		case err := <-fileWatcher.Errors():
			logrus.Debug("logger got error watching file: %v", err)
			// Something happened, let's try and stay alive and create a new watcher
			if retries <= 5 {
				fileWatcher.Close()
				fileWatcher, err = watchFile(name)
				if err != nil {
					return err
				}
				retries++
				return errRetry
			}
			return err
		case <-ctx.Done():
			return errDone
		}
	}

	handleDecodeErr := func(err error) error {
		if errors.Cause(err) != io.EOF {
			return err
		}

		for {
			err := waitRead()
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

	// main loop
	for {
		msg, err := decodeLogLine()
		if err != nil {
			if err := handleDecodeErr(err); err != nil {
				if err == errDone {
					return
				}
				// we got an unrecoverable error, so return
				logWatcher.Err <- err
				return
			}
			// ready to try again
			continue
		}

		retries = 0 // reset retries since we've succeeded
		if !since.IsZero() && msg.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && msg.Timestamp.After(until) {
			return
		}
		select {
		case logWatcher.Msg <- msg:
		case <-ctx.Done():
			logWatcher.Msg <- msg
			for {
				msg, err := decodeLogLine()
				if err != nil {
					return
				}
				if !since.IsZero() && msg.Timestamp.Before(since) {
					continue
				}
				if !until.IsZero() && msg.Timestamp.After(until) {
					return
				}
				logWatcher.Msg <- msg
			}
		}
	}
}

func watchFile(name string) (filenotify.FileWatcher, error) {
	fileWatcher, err := filenotify.New()
	if err != nil {
		return nil, err
	}

	logger := logrus.WithFields(logrus.Fields{
		"module": "logger",
		"fille":  name,
	})

	if err := fileWatcher.Add(name); err != nil {
		logger.WithError(err).Warnf("falling back to file poller")
		fileWatcher.Close()
		fileWatcher = filenotify.NewPollingWatcher()

		if err := fileWatcher.Add(name); err != nil {
			fileWatcher.Close()
			logger.WithError(err).Debugf("error watching log file for modifications")
			return nil, err
		}
	}
	return fileWatcher, nil
}
