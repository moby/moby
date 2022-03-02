package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/filenotify"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/pubsub"
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
		if err != nil && !os.IsNotExist(err) {
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
	closedCh        chan struct{}
	rotateMu        sync.Mutex // blocks the next rotation until the current rotation is completed
	capacity        int64      // maximum size of each file
	currentSize     int64      // current size of the latest file
	maxFiles        int        // maximum number of files
	compress        bool       // whether old versions of log files are compressed
	lastTimestamp   time.Time  // timestamp of the last log
	filesRefCounter refCounter // keep reference-counted of decompressed files
	notifyReaders   *pubsub.Publisher
	marshal         logger.MarshalFunc
	createDecoder   MakeDecoderFn
	getTailReader   GetTailReaderFunc
	perms           os.FileMode
}

// MakeDecoderFn creates a decoder
type MakeDecoderFn func(rdr io.Reader) Decoder

// Decoder is for reading logs
// It is created by the log reader by calling the `MakeDecoderFunc`
type Decoder interface {
	// Reset resets the decoder
	// Reset is called for certain events, such as log rotations
	Reset(io.Reader)
	// Decode decodes the next log messeage from the stream
	Decode() (*logger.Message, error)
	// Close signals to the decoder that it can release whatever resources it was using.
	Close()
}

// SizeReaderAt defines a ReaderAt that also reports its size.
// This is used for tailing log files.
type SizeReaderAt interface {
	io.ReaderAt
	Size() int64
}

// GetTailReaderFunc is used to truncate a reader to only read as much as is required
// in order to get the passed in number of log lines.
// It returns the sectioned reader, the number of lines that the section reader
// contains, and any error that occurs.
type GetTailReaderFunc func(ctx context.Context, f SizeReaderAt, nLogLines int) (rdr io.Reader, nLines int, err error)

// NewLogFile creates new LogFile
func NewLogFile(logPath string, capacity int64, maxFiles int, compress bool, marshaller logger.MarshalFunc, decodeFunc MakeDecoderFn, perms os.FileMode, getTailReader GetTailReaderFunc) (*LogFile, error) {
	log, err := openFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, perms)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		f:               log,
		closedCh:        make(chan struct{}),
		capacity:        capacity,
		currentSize:     size,
		maxFiles:        maxFiles,
		compress:        compress,
		filesRefCounter: refCounter{counter: make(map[string]int)},
		notifyReaders:   pubsub.NewPublisher(0, 1),
		marshal:         marshaller,
		createDecoder:   decodeFunc,
		perms:           perms,
		getTailReader:   getTailReader,
	}, nil
}

// WriteLogEntry writes the provided log message to the current log file.
// This may trigger a rotation event if the max file/capacity limits are hit.
func (w *LogFile) WriteLogEntry(msg *logger.Message) error {
	b, err := w.marshal(msg)
	if err != nil {
		return errors.Wrap(err, "error marshalling log message")
	}

	ts := msg.Timestamp
	logger.PutMessage(msg)
	msg = nil // Turn use-after-put bugs into panics.

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("cannot write because the output file was closed")
	}

	if err := w.checkCapacityAndRotate(); err != nil {
		w.mu.Unlock()
		return errors.Wrap(err, "error rotating log file")
	}

	n, err := w.f.Write(b)
	if err == nil {
		w.currentSize += int64(n)
		w.lastTimestamp = ts
	}

	w.mu.Unlock()
	return errors.Wrap(err, "error writing log entry")
}

func (w *LogFile) checkCapacityAndRotate() (retErr error) {
	if w.capacity == -1 {
		return nil
	}
	if w.currentSize < w.capacity {
		return nil
	}

	w.rotateMu.Lock()
	noCompress := w.maxFiles <= 1 || !w.compress
	defer func() {
		// If we aren't going to run the goroutine to compress the log file, then we need to unlock in this function.
		// Otherwise the lock will be released in the goroutine that handles compression.
		if retErr != nil || noCompress {
			w.rotateMu.Unlock()
		}
	}()

	fname := w.f.Name()
	if err := w.f.Close(); err != nil {
		// if there was an error during a prior rotate, the file could already be closed
		if !errors.Is(err, os.ErrClosed) {
			return errors.Wrap(err, "error closing file")
		}
	}

	if err := rotate(fname, w.maxFiles, w.compress); err != nil {
		logrus.WithError(err).Warn("Error rotating log file, log data may have been lost")
	} else {
		var renameErr error
		for i := 0; i < 10; i++ {
			if renameErr = os.Rename(fname, fname+".1"); renameErr != nil && !os.IsNotExist(renameErr) {
				logrus.WithError(renameErr).WithField("file", fname).Debug("Error rotating current container log file, evicting readers and retrying")
				w.notifyReaders.Publish(renameErr)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			break
		}
		if renameErr != nil {
			logrus.WithError(renameErr).Error("Error renaming current log file")
		}
	}

	file, err := openFile(fname, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, w.perms)
	if err != nil {
		return err
	}
	w.f = file
	w.currentSize = 0

	w.notifyReaders.Publish(struct{}{})

	if noCompress {
		return nil
	}

	ts := w.lastTimestamp

	go func() {
		if err := compressFile(fname+".1", ts); err != nil {
			logrus.WithError(err).Error("Error compressing log file after rotation")
		}
		w.rotateMu.Unlock()
	}()

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
		logrus.WithField("source", fromPath).WithField("target", toPath).Trace("Rotating log file")
		if err := os.Rename(fromPath, toPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func compressFile(fileName string, lastTimestamp time.Time) (retErr error) {
	file, err := open(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.WithField("file", fileName).WithError(err).Debug("Could not open log file to compress")
			return nil
		}
		return errors.Wrap(err, "failed to open log file")
	}
	defer func() {
		file.Close()
		if retErr == nil {
			err := os.Remove(fileName)
			if err != nil && !os.IsNotExist(err) {
				retErr = errors.Wrap(err, "failed to remove source log file")
			}
		}
	}()

	outFile, err := openFile(fileName+".gz", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0640)
	if err != nil {
		return errors.Wrap(err, "failed to open or create gzip log file")
	}
	defer func() {
		outFile.Close()
		if retErr != nil {
			if err := os.Remove(fileName + ".gz"); err != nil && !os.IsExist(err) {
				logrus.WithError(err).Error("Error cleaning up after failed log compression")
			}
		}
	}()

	compressWriter := gzip.NewWriter(outFile)
	defer compressWriter.Close()

	// Add the last log entry timestamp to the gzip header
	extra := rotateFileMetadata{}
	extra.LastTime = lastTimestamp
	compressWriter.Header.Extra, err = json.Marshal(&extra)
	if err != nil {
		// Here log the error only and don't return since this is just an optimization.
		logrus.Warningf("Failed to marshal gzip header as JSON: %v", err)
	}

	_, err = pools.Copy(compressWriter, file)
	if err != nil {
		return errors.Wrapf(err, "error compressing log file %s", fileName)
	}

	return nil
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
	if err := w.f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	w.closed = true
	close(w.closedCh)
	return nil
}

// ReadLogs decodes entries from log files and sends them the passed in watcher
//
// Note: Using the follow option can become inconsistent in cases with very frequent rotations and max log files is 1.
// TODO: Consider a different implementation which can effectively follow logs under frequent rotations.
func (w *LogFile) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	watcher := logger.NewLogWatcher()
	// Lock before starting the reader goroutine to synchronize operations
	// for race-free unit testing. The writer is locked out until the reader
	// has opened the log file and set the read cursor to the current
	// position.
	w.mu.RLock()
	go w.readLogsLocked(config, watcher)
	return watcher
}

func (w *LogFile) readLogsLocked(config logger.ReadConfig, watcher *logger.LogWatcher) {
	defer close(watcher.Msg)

	currentFile, err := open(w.f.Name())
	if err != nil {
		w.mu.RUnlock()
		watcher.Err <- err
		return
	}
	defer currentFile.Close()

	dec := w.createDecoder(nil)
	defer dec.Close()

	currentChunk, err := newSectionReader(currentFile)
	if err != nil {
		w.mu.RUnlock()
		watcher.Err <- err
		return
	}

	notifyEvict := w.notifyReaders.SubscribeTopicWithBuffer(func(i interface{}) bool {
		_, ok := i.(error)
		return ok
	}, 1)
	defer w.notifyReaders.Evict(notifyEvict)

	if config.Tail != 0 {
		// TODO(@cpuguy83): Instead of opening every file, only get the files which
		// are needed to tail.
		// This is especially costly when compression is enabled.
		files, err := w.openRotatedFiles(config)
		w.mu.RUnlock()
		if err != nil {
			watcher.Err <- err
			return
		}

		closeFiles := func() {
			for _, f := range files {
				f.Close()
				fileName := f.Name()
				if strings.HasSuffix(fileName, tmpLogfileSuffix) {
					err := w.filesRefCounter.Dereference(fileName)
					if err != nil {
						logrus.WithError(err).WithField("file", fileName).Error("Failed to dereference the log file")
					}
				}
			}
		}

		readers := make([]SizeReaderAt, 0, len(files)+1)
		for _, f := range files {
			stat, err := f.Stat()
			if err != nil {
				watcher.Err <- errors.Wrap(err, "error reading size of rotated file")
				closeFiles()
				return
			}
			readers = append(readers, io.NewSectionReader(f, 0, stat.Size()))
		}
		if currentChunk.Size() > 0 {
			readers = append(readers, currentChunk)
		}

		ok := tailFiles(readers, watcher, dec, w.getTailReader, config, notifyEvict)
		closeFiles()
		if !ok {
			return
		}
		w.mu.RLock()
	}

	if !config.Follow || w.closed {
		w.mu.RUnlock()
		return
	}
	w.mu.RUnlock()

	notifyRotate := w.notifyReaders.SubscribeTopic(func(i interface{}) bool {
		_, ok := i.(struct{})
		return ok
	})
	defer w.notifyReaders.Evict(notifyRotate)

	followLogs(currentFile, watcher, w.closedCh, notifyRotate, notifyEvict, dec, config.Since, config.Until)
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
					logrus.Warnf("Failed to remove logfile: %v", err)
				}
			}
		}
	}()

	for i := w.maxFiles; i > 1; i-- {
		f, err := open(fmt.Sprintf("%s.%d", w.f.Name(), i-1))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, errors.Wrap(err, "error opening rotated log file")
			}

			fileName := fmt.Sprintf("%s.%d.gz", w.f.Name(), i-1)
			decompressedFileName := fileName + tmpLogfileSuffix
			tmpFile, err := w.filesRefCounter.GetReference(decompressedFileName, func(refFileName string, exists bool) (*os.File, error) {
				if exists {
					return open(refFileName)
				}
				return decompressfile(fileName, refFileName, config.Since)
			})

			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
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
	cf, err := open(fileName)
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

	rs, err := openFile(destFileName, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return nil, errors.Wrap(err, "error creating file for copying decompressed log stream")
	}

	_, err = pools.Copy(rs, rc)
	if err != nil {
		rs.Close()
		rErr := os.Remove(rs.Name())
		if rErr != nil && !os.IsNotExist(rErr) {
			logrus.Errorf("Failed to remove logfile: %v", rErr)
		}
		return nil, errors.Wrap(err, "error while copying decompressed log stream to file")
	}

	return rs, nil
}

func newSectionReader(f *os.File) (*io.SectionReader, error) {
	// seek to the end to get the size
	// we'll leave this at the end of the file since section reader does not advance the reader
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, errors.Wrap(err, "error getting current file size")
	}
	return io.NewSectionReader(f, 0, size), nil
}

func tailFiles(files []SizeReaderAt, watcher *logger.LogWatcher, dec Decoder, getTailReader GetTailReaderFunc, config logger.ReadConfig, notifyEvict <-chan interface{}) (cont bool) {
	nLines := config.Tail

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cont = true
	// TODO(@cpuguy83): we should plumb a context through instead of dealing with `WatchClose()` here.
	go func() {
		select {
		case err := <-notifyEvict:
			if err != nil {
				watcher.Err <- err.(error)
				cont = false
				cancel()
			}
		case <-ctx.Done():
		case <-watcher.WatchConsumerGone():
			cont = false
			cancel()
		}
	}()

	readers := make([]io.Reader, 0, len(files))

	if config.Tail > 0 {
		for i := len(files) - 1; i >= 0 && nLines > 0; i-- {
			tail, n, err := getTailReader(ctx, files[i], nLines)
			if err != nil {
				watcher.Err <- errors.Wrap(err, "error finding file position to start log tailing")
				return
			}
			nLines -= n
			readers = append([]io.Reader{tail}, readers...)
		}
	} else {
		for _, r := range files {
			readers = append(readers, &wrappedReaderAt{ReaderAt: r})
		}
	}

	rdr := io.MultiReader(readers...)
	dec.Reset(rdr)

	for {
		msg, err := dec.Decode()
		if err != nil {
			if !errors.Is(err, io.EOF) {
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
		case <-ctx.Done():
			return
		case watcher.Msg <- msg:
		}
	}
}

func watchFile(name string) (filenotify.FileWatcher, error) {
	var fileWatcher filenotify.FileWatcher

	if runtime.GOOS == "windows" {
		// FileWatcher on Windows files is based on the syscall notifications which has an issue because of file caching.
		// It is based on ReadDirectoryChangesW() which doesn't detect writes to the cache. It detects writes to disk only.
		// Because of the OS lazy writing, we don't get notifications for file writes and thereby the watcher
		// doesn't work. Hence for Windows we will use poll based notifier.
		fileWatcher = filenotify.NewPollingWatcher()
	} else {
		var err error
		fileWatcher, err = filenotify.New()
		if err != nil {
			return nil, err
		}
	}

	logger := logrus.WithFields(logrus.Fields{
		"module": "logger",
		"file":   name,
	})

	if err := fileWatcher.Add(name); err != nil {
		// we will retry using file poller.
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

type wrappedReaderAt struct {
	io.ReaderAt
	pos int64
}

func (r *wrappedReaderAt) Read(p []byte) (int, error) {
	n, err := r.ReaderAt.ReadAt(p, r.pos)
	r.pos += int64(n)
	return n, err
}
