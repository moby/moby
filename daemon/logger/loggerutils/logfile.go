package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
)

// rotateFileMetadata is a metadata of the gzip header of the compressed log file
type rotateFileMetadata struct {
	LastTime time.Time `json:"lastTime,omitempty"`
}

// LogFile is Logger implementation for default Docker logging.
type LogFile struct {
	mu       sync.Mutex // protects the logfile access
	closed   chan struct{}
	rotateMu sync.Mutex // blocks the next rotation until the current rotation is completed
	// Lock out readers while performing a non-atomic sequence of filesystem
	// operations (RLock: open, Lock: rename, delete).
	//
	// fsopMu should be locked for writing only while holding rotateMu.
	fsopMu sync.RWMutex

	// Logger configuration

	capacity int64 // maximum size of each file
	maxFiles int   // maximum number of files
	compress bool  // whether old versions of log files are compressed
	perms    os.FileMode

	// Log file codec

	createDecoder MakeDecoderFn
	getTailReader GetTailReaderFunc

	// Log reader state in a 1-buffered channel.
	//
	// Share memory by communicating: receive to acquire, send to release.
	// The state struct is passed around by value so that use-after-send
	// bugs cannot escalate to data races.
	//
	// A method which receives the state value takes ownership of it. The
	// owner is responsible for either passing ownership along or sending
	// the state back to the channel. By convention, the semantics of
	// passing along ownership is expressed with function argument types.
	// Methods which take a pointer *logReadState argument borrow the state,
	// analogous to functions which require a lock to be held when calling.
	// The caller retains ownership. Calling a method which which takes a
	// value logFileState argument gives ownership to the callee.
	read chan logReadState

	decompress *sharedTempFileConverter

	pos           logPos    // Current log file write position.
	f             *os.File  // Current log file for writing.
	lastTimestamp time.Time // timestamp of the last log
}

type logPos struct {
	// Size of the current file.
	size int64
	// File rotation sequence number (modulo 2**16).
	rotation uint16
}

type logReadState struct {
	// Current log file position.
	pos logPos
	// Wait list to be notified of the value of pos next time it changes.
	wait []chan<- logPos
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
	io.Reader
	io.ReaderAt
	Size() int64
}

type readAtCloser interface {
	io.ReaderAt
	io.Closer
}

// GetTailReaderFunc is used to truncate a reader to only read as much as is required
// in order to get the passed in number of log lines.
// It returns the sectioned reader, the number of lines that the section reader
// contains, and any error that occurs.
type GetTailReaderFunc func(ctx context.Context, f SizeReaderAt, nLogLines int) (rdr io.Reader, nLines int, err error)

// NewLogFile creates new LogFile
func NewLogFile(logPath string, capacity int64, maxFiles int, compress bool, decodeFunc MakeDecoderFn, perms os.FileMode, getTailReader GetTailReaderFunc) (*LogFile, error) {
	log, err := openFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, perms)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	pos := logPos{
		size: size,
		// Force a wraparound on first rotation to shake out any
		// modular-arithmetic bugs.
		rotation: math.MaxUint16,
	}
	st := make(chan logReadState, 1)
	st <- logReadState{pos: pos}

	return &LogFile{
		f:             log,
		read:          st,
		pos:           pos,
		closed:        make(chan struct{}),
		capacity:      capacity,
		maxFiles:      maxFiles,
		compress:      compress,
		decompress:    newSharedTempFileConverter(decompress),
		createDecoder: decodeFunc,
		perms:         perms,
		getTailReader: getTailReader,
	}, nil
}

// WriteLogEntry writes the provided log message to the current log file.
// This may trigger a rotation event if the max file/capacity limits are hit.
func (w *LogFile) WriteLogEntry(timestamp time.Time, marshalled []byte) error {
	select {
	case <-w.closed:
		return errors.New("cannot write because the output file was closed")
	default:
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	// Are we due for a rotation?
	if w.capacity != -1 && w.pos.size >= w.capacity {
		if err := w.rotate(); err != nil {
			return errors.Wrap(err, "error rotating log file")
		}
	}

	n, err := w.f.Write(marshalled)
	if err != nil {
		return errors.Wrap(err, "error writing log entry")
	}
	w.pos.size += int64(n)
	w.lastTimestamp = timestamp

	// Notify any waiting readers that there is a new log entry to read.
	st := <-w.read
	defer func() { w.read <- st }()
	st.pos = w.pos

	for _, c := range st.wait {
		c <- st.pos
	}
	// Optimization: retain the backing array to save a heap allocation next
	// time a reader appends to the list.
	if st.wait != nil {
		st.wait = st.wait[:0]
	}
	return nil
}

func (w *LogFile) rotate() (retErr error) {
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
		if !errors.Is(err, fs.ErrClosed) {
			return errors.Wrap(err, "error closing file")
		}
	}

	file, err := func() (*os.File, error) {
		w.fsopMu.Lock()
		defer w.fsopMu.Unlock()

		if err := rotate(fname, w.maxFiles, w.compress); err != nil {
			log.G(context.TODO()).WithError(err).Warn("Error rotating log file, log data may have been lost")
		} else {
			// We may have readers working their way through the
			// current log file so we can't truncate it. We need to
			// start writing new logs to an empty file with the same
			// name as the current one so we need to rotate the
			// current file out of the way.
			if w.maxFiles < 2 {
				if err := unlink(fname); err != nil && !errors.Is(err, fs.ErrNotExist) {
					log.G(context.TODO()).WithError(err).Error("Error unlinking current log file")
				}
			} else {
				if err := os.Rename(fname, fname+".1"); err != nil && !errors.Is(err, fs.ErrNotExist) {
					log.G(context.TODO()).WithError(err).Error("Error renaming current log file")
				}
			}
		}

		// Notwithstanding the above, open with the truncate flag anyway
		// in case rotation didn't work out as planned.
		return openFile(fname, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, w.perms)
	}()
	if err != nil {
		return err
	}
	w.f = file
	w.pos = logPos{rotation: w.pos.rotation + 1}

	if noCompress {
		return nil
	}

	ts := w.lastTimestamp
	go func() {
		defer w.rotateMu.Unlock()
		// No need to hold fsopMu as at no point will the filesystem be
		// in a state which would cause problems for readers. Opening
		// the uncompressed file is tried first, falling back to the
		// compressed one. compressFile only deletes the uncompressed
		// file once the compressed one is fully written out, so at no
		// point during the compression process will a reader fail to
		// open a complete copy of the file.
		if err := compressFile(fname+".1", ts); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error compressing log file after rotation")
		}
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
	err := unlink(lastFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Wrap(err, "error removing oldest log file")
	}

	for i := maxFiles - 1; i > 1; i-- {
		toPath := name + "." + strconv.Itoa(i) + extension
		fromPath := name + "." + strconv.Itoa(i-1) + extension
		err := os.Rename(fromPath, toPath)
		log.G(context.TODO()).WithError(err).WithField("source", fromPath).WithField("target", toPath).Trace("Rotating log file")
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

func compressFile(fileName string, lastTimestamp time.Time) (retErr error) {
	file, err := open(fileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.G(context.TODO()).WithField("file", fileName).WithError(err).Debug("Could not open log file to compress")
			return nil
		}
		return errors.Wrap(err, "failed to open log file")
	}
	defer func() {
		file.Close()
		if retErr == nil {
			err := unlink(fileName)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				retErr = errors.Wrap(err, "failed to remove source log file")
			}
		}
	}()

	outFile, err := openFile(fileName+".gz", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o640)
	if err != nil {
		return errors.Wrap(err, "failed to open or create gzip log file")
	}
	defer func() {
		outFile.Close()
		if retErr != nil {
			if err := unlink(fileName + ".gz"); err != nil && !errors.Is(err, fs.ErrNotExist) {
				log.G(context.TODO()).WithError(err).Error("Error cleaning up after failed log compression")
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
		log.G(context.TODO()).Warningf("Failed to marshal gzip header as JSON: %v", err)
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
	select {
	case <-w.closed:
		return nil
	default:
	}
	if err := w.f.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
		return err
	}
	close(w.closed)
	// Wait until any in-progress rotation is complete.
	w.rotateMu.Lock()
	w.rotateMu.Unlock() //nolint:staticcheck
	return nil
}

// ReadLogs decodes entries from log files.
//
// It is the caller's responsibility to call ConsumerGone on the LogWatcher.
func (w *LogFile) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	watcher := logger.NewLogWatcher()
	// Lock out filesystem operations so that we can capture the read
	// position and atomically open the corresponding log file, without the
	// file getting rotated out from under us.
	w.fsopMu.RLock()
	// Capture the read position synchronously to ensure that we start
	// following from the last entry logged before ReadLogs was called,
	// which is required for flake-free unit testing.
	st := <-w.read
	pos := st.pos
	w.read <- st
	go w.readLogsLocked(pos, config, watcher)
	return watcher
}

// readLogsLocked is the bulk of the implementation of ReadLogs.
//
// w.fsopMu must be locked for reading when calling this method.
// w.fsopMu.RUnlock() is called before returning.
func (w *LogFile) readLogsLocked(currentPos logPos, config logger.ReadConfig, watcher *logger.LogWatcher) {
	defer close(watcher.Msg)

	currentFile, err := open(w.f.Name())
	if err != nil {
		watcher.Err <- err
		return
	}
	defer currentFile.Close()

	dec := w.createDecoder(nil)
	defer dec.Close()

	currentChunk := io.NewSectionReader(currentFile, 0, currentPos.size)
	fwd := newForwarder(config)

	if config.Tail != 0 {
		// TODO(@cpuguy83): Instead of opening every file, only get the files which
		// are needed to tail.
		// This is especially costly when compression is enabled.
		files, err := w.openRotatedFiles(config)
		if err != nil {
			watcher.Err <- err
			return
		}

		closeFiles := func() {
			for _, f := range files {
				f.Close()
			}
		}

		readers := make([]SizeReaderAt, 0, len(files)+1)
		for _, f := range files {
			switch ff := f.(type) {
			case SizeReaderAt:
				readers = append(readers, ff)
			case interface{ Stat() (fs.FileInfo, error) }:
				stat, err := ff.Stat()
				if err != nil {
					watcher.Err <- errors.Wrap(err, "error reading size of rotated file")
					closeFiles()
					return
				}
				readers = append(readers, io.NewSectionReader(f, 0, stat.Size()))
			default:
				panic(fmt.Errorf("rotated file value %#v (%[1]T) has neither Size() nor Stat() methods", f))
			}
		}
		if currentChunk.Size() > 0 {
			readers = append(readers, currentChunk)
		}

		ok := tailFiles(readers, watcher, dec, w.getTailReader, config.Tail, fwd)
		closeFiles()
		if !ok {
			return
		}
	} else {
		w.fsopMu.RUnlock()
	}

	if !config.Follow {
		return
	}

	(&follow{
		LogFile:   w,
		Watcher:   watcher,
		Decoder:   dec,
		Forwarder: fwd,
	}).Do(currentFile, currentPos)
}

// openRotatedFiles returns a slice of files open for reading, in order from
// oldest to newest, and calls w.fsopMu.RUnlock() before returning.
//
// This method must only be called with w.fsopMu locked for reading.
func (w *LogFile) openRotatedFiles(config logger.ReadConfig) (files []readAtCloser, err error) {
	type rotatedFile struct {
		f          *os.File
		compressed bool
	}

	var q []rotatedFile
	defer func() {
		if err != nil {
			for _, qq := range q {
				qq.f.Close()
			}
			for _, f := range files {
				f.Close()
			}
		}
	}()

	q, err = func() (q []rotatedFile, err error) {
		defer w.fsopMu.RUnlock()

		q = make([]rotatedFile, 0, w.maxFiles)
		for i := w.maxFiles; i > 1; i-- {
			var f rotatedFile
			f.f, err = open(fmt.Sprintf("%s.%d", w.f.Name(), i-1))
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return nil, errors.Wrap(err, "error opening rotated log file")
				}
				f.compressed = true
				f.f, err = open(fmt.Sprintf("%s.%d.gz", w.f.Name(), i-1))
				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						return nil, errors.Wrap(err, "error opening file for decompression")
					}
					continue
				}
			}
			q = append(q, f)
		}
		return q, nil
	}()
	if err != nil {
		return nil, err
	}

	for len(q) > 0 {
		qq := q[0]
		q = q[1:]
		if qq.compressed {
			defer qq.f.Close()
			f, err := w.maybeDecompressFile(qq.f, config)
			if err != nil {
				return nil, err
			}
			if f != nil {
				// The log before `config.Since` does not need to read
				files = append(files, f)
			}
		} else {
			files = append(files, qq.f)
		}
	}
	return files, nil
}

func (w *LogFile) maybeDecompressFile(cf *os.File, config logger.ReadConfig) (readAtCloser, error) {
	rc, err := gzip.NewReader(cf)
	if err != nil {
		return nil, errors.Wrap(err, "error making gzip reader for compressed log file")
	}
	defer rc.Close()

	// Extract the last log entry timestramp from the gzip header
	extra := &rotateFileMetadata{}
	err = json.Unmarshal(rc.Header.Extra, extra)
	if err == nil && !extra.LastTime.IsZero() && extra.LastTime.Before(config.Since) {
		return nil, nil
	}
	tmpf, err := w.decompress.Do(cf)
	return tmpf, errors.Wrap(err, "error decompressing log file")
}

func decompress(dst io.WriteSeeker, src io.ReadSeeker) error {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	rc, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	_, err = pools.Copy(dst, rc)
	if err != nil {
		return err
	}
	return rc.Close()
}

func tailFiles(files []SizeReaderAt, watcher *logger.LogWatcher, dec Decoder, getTailReader GetTailReaderFunc, nLines int, fwd *forwarder) (cont bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cont = true
	// TODO(@cpuguy83): we should plumb a context through instead of dealing with `WatchClose()` here.
	go func() {
		select {
		case <-ctx.Done():
		case <-watcher.WatchConsumerGone():
			cancel()
		}
	}()

	readers := make([]io.Reader, 0, len(files))

	if nLines > 0 {
		for i := len(files) - 1; i >= 0 && nLines > 0; i-- {
			tail, n, err := getTailReader(ctx, files[i], nLines)
			if err != nil {
				watcher.Err <- errors.Wrap(err, "error finding file position to start log tailing")
				return false
			}
			nLines -= n
			readers = append([]io.Reader{tail}, readers...)
		}
	} else {
		for _, r := range files {
			readers = append(readers, r)
		}
	}

	rdr := io.MultiReader(readers...)
	dec.Reset(rdr)
	return fwd.Do(watcher, dec)
}

type forwarder struct {
	since, until time.Time
}

func newForwarder(config logger.ReadConfig) *forwarder {
	return &forwarder{since: config.Since, until: config.Until}
}

// Do reads log messages from dec and sends the messages matching the filter
// conditions to watcher. Do returns cont=true iff it has read all messages from
// dec without encountering a message with a timestamp which is after the
// configured until time.
func (fwd *forwarder) Do(watcher *logger.LogWatcher, dec Decoder) (cont bool) {
	for {
		msg, err := dec.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return true
			}
			watcher.Err <- err
			return false
		}
		if !fwd.since.IsZero() {
			if msg.Timestamp.Before(fwd.since) {
				continue
			}
			// We've found our first message with a timestamp >= since. As message
			// timestamps might not be monotonic, we need to skip the since check for all
			// subsequent messages so we do not filter out later messages which happen to
			// have timestamps before since.
			fwd.since = time.Time{}
		}
		if !fwd.until.IsZero() && msg.Timestamp.After(fwd.until) {
			return false
		}
		select {
		case <-watcher.WatchConsumerGone():
			return false
		case watcher.Msg <- msg:
		}
	}
}
