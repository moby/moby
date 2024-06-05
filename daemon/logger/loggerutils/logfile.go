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
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/tracing"
	"github.com/containerd/log"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	// decomrpessMu is used to prevent log readers from trying to read decompressed
	// log files while decompression is still in progress
	decompressMu sync.Mutex

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
	// The caller retains ownership. Calling a method which takes a
	// value logFileState argument gives ownership to the callee.
	read chan logReadState

	pos           logPos    // Current log file write position.
	f             *os.File  // Current log file for writing.
	lastTimestamp time.Time // timestamp of the last log

	decompressTmpDir string
	activeDecompress *refCounter
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

// SyntaxError is an error that should be returend by a [Decoder] when it is
// unable to parse the data.
type SyntaxError struct {
	// Err is the underlying error
	Err error
	// Offset is the position in a decode stream where the content could not be
	// parsed from
	Offset int64
}

func (e *SyntaxError) Error() string {
	return e.Err.Error()
}

func (e *SyntaxError) Unwrap() error {
	return e.Err
}

// Decoder is for reading logs
// It is created by the log reader by calling the `MakeDecoderFunc`
type Decoder interface {
	// Reset resets the decoder
	// Reset is called for certain events, such as log rotations
	Reset(io.Reader)
	// Decode decodes the next log messeage from the stream
	//
	// When the decoder is unable to parse a message it should wrap the underlying
	// error in a [SyntaxError]
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

// GetTailReaderFunc is used to truncate a reader to only read as much as is required
// in order to get the passed in number of log lines.
// It returns the sectioned reader, the number of lines that the section reader
// contains, and any error that occurs.
type GetTailReaderFunc func(ctx context.Context, f SizeReaderAt, nLogLines int) (rdr SizeReaderAt, nLines int, err error)

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

	if w.decompressTmpDir != "" {
		os.RemoveAll(w.decompressTmpDir)
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
//
// The provided context should only be used for passing contextual information
// such as tracing spans and loggers.
// It should not be used for cancellation.
func (w *LogFile) ReadLogs(ctx context.Context, config logger.ReadConfig) *logger.LogWatcher {
	ctx, span := tracing.StartSpan(ctx, "LogFile.ReadLogs")
	span.SetAttributes(tracing.Attribute("config", config))

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
	go w.readLogsLocked(ctx, pos, config, watcher)
	return watcher
}

func (w *LogFile) getTailFiles(ctx context.Context, config logger.ReadConfig) (_ []SizeReaderAt, _ func(context.Context), retErr error) {
	// TODO(@cpuguy83): Instead of opening every file, only get the files which
	// are needed to tail.
	// This is especially costly when compression is enabled.
	files, release, err := w.openRotatedFiles(ctx, config)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func(ctx context.Context) {
		for _, f := range files {
			f.Close()
		}
		release(ctx)
	}

	defer func() {
		if retErr != nil {
			cleanup(ctx)
		}
	}()

	readers := make([]SizeReaderAt, 0, len(files))
	for _, f := range files {
		stat, err := f.Stat()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error getting size of log file: %s", f.Name())
		}
		readers = append(readers, io.NewSectionReader(f, 0, stat.Size()))
	}

	return readers, cleanup, nil
}

// readLogsLocked is the bulk of the implementation of ReadLogs.
//
// w.fsopMu must be locked for reading when calling this method.
// w.fsopMu.RUnlock() is called before returning.
func (w *LogFile) readLogsLocked(ctx context.Context, currentPos logPos, config logger.ReadConfig, watcher *logger.LogWatcher) {
	span := tracing.SpanFromContext(ctx)
	defer span.End()

	defer close(watcher.Msg)

	currentFile, err := open(w.f.Name())
	if err != nil {
		span.SetStatus(err)
		watcher.Err <- err
		return
	}
	defer currentFile.Close()

	dec := w.createDecoder(nil)
	defer dec.Close()

	fwd := newForwarder(config)

	if config.Tail != 0 {
		span.AddEvent("Tail logs")
		files, release, err := w.getTailFiles(ctx, config)
		w.fsopMu.RUnlock()
		if err != nil {
			watcher.Err <- err
			span.SetStatus(err)
			return
		}

		currentChunk := io.NewSectionReader(currentFile, 0, currentPos.size)
		if currentChunk.Size() > 0 {
			files = append(files, currentChunk)
		}

		if !tailFiles(ctx, files, watcher, dec, w.getTailReader, config.Tail, fwd) {
			span.AddEvent("Done tailing logs", trace.WithAttributes(attribute.Bool("continue", false)))
			release(ctx)
			return
		}
		release(ctx)
		span.AddEvent("Done tailing logs")
	} else {
		w.fsopMu.RUnlock()
	}

	if !config.Follow {
		return
	}

	span.AddEvent("Follow logs")

	(&follow{
		LogFile:   w,
		Watcher:   watcher,
		Decoder:   dec,
		Forwarder: fwd,
	}).Do(currentFile, currentPos)

	span.AddEvent("Done following logs")
}

// openRotatedFiles returns a slice of files open for reading, in order from
// oldest to newest, and calls w.fsopMu.RUnlock() before returning.
//
// This method must only be called with w.fsopMu locked for reading.
func (w *LogFile) openRotatedFiles(ctx context.Context, config logger.ReadConfig) (_ []*os.File, _ func(context.Context), retErr error) {
	ctx, span := tracing.StartSpan(ctx, "openRotatedFiles")
	defer func() {
		if retErr != nil {
			span.SetStatus(retErr)
		}
		span.End()
	}()

	var (
		files    []*os.File
		cleanups []func(context.Context)
	)
	release := func(ctx context.Context) {
		for _, f := range cleanups {
			f(ctx)
		}
	}

	defer func() {
		if retErr != nil {
			for _, f := range files {
				f.Close()
			}
			release(ctx)
		}
	}()

	for i := w.maxFiles; i > 1; i-- {
		f, err := open(fmt.Sprintf("%s.%d", w.f.Name(), i-1))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, nil, errors.Wrap(err, "error opening rotated log file")
			}

			cf, err := open(fmt.Sprintf("%s.%d.gz", w.f.Name(), i-1))
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return nil, nil, errors.Wrap(err, "error opening file for decompression")
				}
				continue
			}

			f2, cleanup, err := w.maybeDecompressFile(ctx, cf, config)
			if err != nil {
				return nil, nil, err
			}
			if f2 != nil {
				cleanups = append(cleanups, cleanup)
				f = f2
			}
		}
		if f != nil {
			files = append(files, f)
		}
	}

	return files, release, nil
}

type refCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func (c *refCounter) Inc(key string) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	active, ok := c.counts[key]
	if !ok {
		if c.counts == nil {
			c.counts = map[string]int{}
		}
		active = 0
	}

	active++

	c.counts[key] = active

	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()

		active := c.counts[key]
		active--

		if active == 0 {
			delete(c.counts, key)
			return true
		}

		c.counts[key] = active

		return false
	}
}

func (w *LogFile) maybeDecompressFile(ctx context.Context, cf *os.File, config logger.ReadConfig) (_ *os.File, _ func(context.Context), retErr error) {
	ctx, span := tracing.StartSpan(ctx, "maybeDecompressFile")
	span.SetAttributes(attribute.String("file", cf.Name()))

	defer func() {
		if retErr != nil {
			span.SetStatus(retErr)
		}
		span.End()
	}()

	rc, err := gzip.NewReader(cf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error making gzip reader for compressed log file")
	}
	defer rc.Close()

	// Extract the last log entry timestramp from the gzip header
	extra := &rotateFileMetadata{}
	err = json.Unmarshal(rc.Header.Extra, extra)
	if err == nil {
		span.SetAttributes(attribute.Stringer("lastEvent", extra.LastTime))
		if !extra.LastTime.IsZero() && extra.LastTime.Before(config.Since) {
			span.AddEvent("skip decompressing log file with messages not included in time period")
			return nil, nil, nil
		}
	}

	w.decompressMu.Lock()
	defer w.decompressMu.Unlock()
	span.AddEvent("have decompress lock")

	if w.decompressTmpDir == "" {
		span.AddEvent("create decompress dir")
		p, err := os.MkdirTemp("", "decompressed-logs")
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not create temp dir for decompressed logs")
		}
		w.decompressTmpDir = p
	}

	if w.activeDecompress == nil {
		w.activeDecompress = &refCounter{}
	}

	// We'll use the last time entry in the log file to determine the filename to use for decompression
	t := extra.LastTime.UnixNano()
	p := filepath.Join(w.decompressTmpDir, strconv.Itoa(int(t))+".log")
	span.SetAttributes(attribute.String("decompressed", p))

	dec := w.activeDecompress.Inc(p)

	release := func(ctx context.Context) {
		// only need to lock this if we made it out of the main function
		if retErr == nil {
			w.decompressMu.Lock()
			defer w.decompressMu.Unlock()
		}

		if dec() {
			span := trace.SpanFromContext(ctx)
			if span != nil {
				span.AddEvent("Release file", trace.WithAttributes(attribute.String("file", p)))
			}
			os.Remove(p)
		}
	}

	defer func() {
		if retErr != nil {
			release(ctx)
		}
	}()

	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not open decompressed log file for %s", cf.Name())
	}

	defer func() {
		if retErr != nil {
			f.Close()
		}
	}()

	stat, err := f.Stat()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error stating decompressed log file: %s", f.Name())
	}

	if stat.Size() > 0 {
		// note: we are currently holding the decompression lock.
		// Since the lock is held while decompression is in progress we know that
		// if we were able to get here that decompression must be completed already.
		return f, release, nil
	}

	cStat, err := cf.Stat()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error stating compresed file: %s", cf.Name())
	}

	if err := decompress(f, io.NewSectionReader(cf, 0, cStat.Size())); err != nil {
		return nil, nil, errors.Wrap(err, "error decompressing log file")
	}
	return f, release, nil
}

func decompress(dst io.Writer, src io.Reader) error {
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

func (e *SyntaxError) String() string {
	return e.Err.Error()
}

func tailFiles(ctx context.Context, files []SizeReaderAt, watcher *logger.LogWatcher, dec Decoder, getTailReader GetTailReaderFunc, nLines int, fwd *forwarder) (cont bool) {
	ctx, span := tracing.StartSpan(ctx, "tailFiles")
	defer func() {
		span.SetAttributes(attribute.Bool("continue", cont))
		span.End()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO(@cpuguy83): we should plumb a context through instead of dealing with `WatchClose()` here.
	go func() {
		select {
		case <-ctx.Done():
		case <-watcher.WatchConsumerGone():
			cancel()
		}
	}()

	consume := func(rdr SizeReaderAt) bool {
		for {
			dec.Reset(rdr)
			cont, err := fwd.Do(ctx, watcher, dec)
			if err != nil {
				if err.Offset < rdr.Size() {
					// The log file may have some corruption in it.
					// Try advancing beyond the failed offset and try again
					rdr = io.NewSectionReader(rdr, err.Offset+1, rdr.Size()-err.Offset+1)
					span.AddEvent("Advanced reader due to syntax error", trace.WithAttributes(
						attribute.Int64("error at offset", err.Offset),
						attribute.Stringer("error", err),
					))
					continue
				}
				// We've reached the end of the file, nothing else we can really do here
				// Except move on to the next file.
			}
			return cont
		}
	}

	if nLines <= 0 {
		for _, rdr := range files {
			if !consume(rdr) {
				return false
			}
		}
		return true
	}

	tailFiles := make([]SizeReaderAt, 0, len(files))
	for i := len(files) - 1; i >= 0 && nLines > 0; i-- {
		tail, n, err := getTailReader(ctx, files[i], nLines)
		if err != nil {
			err := errors.Wrap(err, "error finding file position to start log tailing")
			span.SetStatus(err)
			watcher.Err <- err
			return
		}

		span.AddEvent("Add file to tail", trace.WithAttributes(
			attribute.Int("Number of lines to tail from file", n),
			attribute.Int("File index", i),
		))
		nLines -= n
		tailFiles = append(tailFiles, tail)
	}

	slices.Reverse(tailFiles)

	for _, rdr := range tailFiles {
		if !consume(rdr) {
			return false
		}
	}

	return true
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
//
// Currently the only error condition returned by this function is when there is
// a [SyntaxError] during decode.
// When this occurs this looks like the logs got corrupted somewhow, such as
// power failure before fsync.
// In this case it is expected that the caller advances the the underlying reader
// and tries again.
// Because of the nature of the error this could end up being many retries (perhaps up to EOF).
//
// note: this is inteiontally returning a specific error type instead of the error interface
// because this should only ever return *SyntaxError as an error (including the nil value).
func (fwd *forwarder) Do(ctx context.Context, watcher *logger.LogWatcher, dec Decoder) (cont bool, _ *SyntaxError) {
	ctx, span := tracing.StartSpan(ctx, "forwarder.Do")
	defer span.End()

	defer func() {
		span.SetAttributes(attribute.Bool("continue", cont))
	}()

	for {
		msg, err := dec.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return true, nil
			}

			span.SetStatus(err)

			var sErr *SyntaxError
			if errors.As(err, &sErr) {
				return true, sErr
			}

			watcher.Err <- err
			return false, nil
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
			return false, nil
		}
		select {
		case <-watcher.WatchConsumerGone():
			return false, nil
		case watcher.Msg <- msg:
		}
	}
}
