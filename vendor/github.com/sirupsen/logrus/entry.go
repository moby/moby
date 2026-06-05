package logrus

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (

	// qualified package name, cached at first use
	logrusPackage string

	// Positions in the call stack when tracing to report the calling method.
	//
	// Start at the bottom of the stack before the package-name cache is primed.
	minimumCallerDepth = 1

	// Used for caller information initialisation
	callerInitOnce sync.Once
)

const (
	maximumCallerDepth int = 25
	knownLogrusFrames  int = 4
)

// ErrorKey defines the key when adding errors using [WithError], [Logger.WithError].
var ErrorKey = "error"

// Entry represents a single log event. It may be either an intermediate
// entry (created via WithField(s), WithContext, etc.) or a final entry
// that is emitted when one of the level methods (Trace, Debug, Info,
// Warn, Error, Fatal, Panic) is called.
//
// An Entry always belongs to a Logger. A nil Logger is invalid and will
// cause a panic when the entry is logged. Use [NewEntry] or Logger methods
// to construct entries.
//
// Entries are safe to reuse for adding fields and may be passed around
// to avoid field duplication. Each log operation operates on a copy
// of the Entry’s data to avoid mutation during formatting.
//
//nolint:recvcheck // Entry methods intentionally use both pointer and value receivers.
type Entry struct {
	// Logger is the Logger that owns this entry and is responsible for
	// formatting, hooks, and output. It must not be nil. An Entry without
	// a Logger is invalid and will panic when logged.
	Logger *Logger

	// Data contains all user-defined fields attached to this entry.
	Data Fields

	// Time is the timestamp for the log event. If zero when the entry is
	// logged, it defaults to the current time.
	Time time.Time

	// Level is the severity of the log entry. It is set when the entry
	// is fired and reflects the level used for that log call.
	Level Level

	// Caller contains the calling method information when caller
	// reporting is enabled.
	Caller *runtime.Frame

	// Message is the log message supplied to one of the logging methods
	// (Trace, Debug, Info, Warn, Error, Fatal, or Panic). It is set when
	// the entry is logged.
	Message string

	// Buffer is a reusable buffer provided to the formatter. It is set
	// before formatting in the normal log path; when nil, formatters
	// allocate their own.
	Buffer *bytes.Buffer

	// Context carries user-provided context for hooks and formatters.
	Context context.Context

	// err contains internal field-formatting errors.
	err string
}

// NewEntry creates a new [Entry] associated with the provided Logger.
// The logger must not be nil. Passing a nil logger results in a
// panic when a logging method (e.g., [Entry.Info], [Entry.Error], etc.)
// is called.
func NewEntry(logger *Logger) *Entry {
	return &Entry{
		Logger: logger,
		// Reserve default predefined fields and a little extra room.
		Data: make(Fields, defaultFields+3),
	}
}

// Dup creates a copy of the entry for further modification.
//
// Data is cloned to avoid mutating the original entry. Other fields
// (Logger, Time, Context, etc.) are copied by value.
func (entry *Entry) Dup() *Entry {
	return &Entry{
		Logger:  entry.Logger,
		Data:    maps.Clone(entry.Data),
		Time:    entry.Time,
		Context: entry.Context,
		err:     entry.err,
	}
}

// Bytes returns the bytes representation of this entry from the formatter.
func (entry *Entry) Bytes() ([]byte, error) {
	// Snapshot the formatter under the lock to protect against concurrent
	// SetFormatter calls, then release the lock before formatting.
	// This avoids a data race and prevents a deadlock if Format() triggers
	// reentrant logging (e.g., a field's MarshalJSON calls logrus).
	//
	// See:
	//
	// - https://github.com/sirupsen/logrus/issues/1440
	// - https://github.com/sirupsen/logrus/issues/1448
	entry.Logger.mu.Lock()
	formatter := entry.Logger.Formatter
	entry.Logger.mu.Unlock()

	return formatter.Format(entry)
}

// String returns the string representation from the reader and ultimately the
// formatter.
func (entry *Entry) String() (string, error) {
	serialized, err := entry.Bytes()
	if err != nil {
		return "", err
	}
	str := string(serialized)
	return str, nil
}

// WithError adds an error as single field (using the key defined in [ErrorKey])
// to the Entry.
func (entry *Entry) WithError(err error) *Entry {
	// Avoid reflection work in WithFields; we know the type is an error;
	// copy the entry data and set the ErrorKey directly.
	data := make(Fields, len(entry.Data)+1)
	maps.Copy(data, entry.Data)
	data[ErrorKey] = err

	return &Entry{
		Logger:  entry.Logger,
		Data:    data,
		Time:    entry.Time,
		Context: entry.Context,
		err:     entry.err,
	}
}

// WithContext adds a context to the Entry.
func (entry *Entry) WithContext(ctx context.Context) *Entry {
	return &Entry{
		Logger:  entry.Logger,
		Data:    maps.Clone(entry.Data),
		Time:    entry.Time,
		Context: ctx,
		err:     entry.err,
	}
}

// WithField adds a single field to the Entry.
func (entry *Entry) WithField(key string, value any) *Entry {
	return entry.WithFields(Fields{key: value})
}

// WithFields adds a map of fields to the Entry.
func (entry *Entry) WithFields(fields Fields) *Entry {
	data := make(Fields, len(entry.Data)+len(fields))
	maps.Copy(data, entry.Data)
	fieldErr := entry.err
	for k, v := range fields {
		isErrField := false
		if t := reflect.TypeOf(v); t != nil {
			switch {
			case t.Kind() == reflect.Func, t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Func:
				isErrField = true
			}
		}
		if isErrField {
			tmp := fmt.Sprintf("can not add field %q", k)
			if fieldErr != "" {
				fieldErr += ", " + tmp
			} else {
				fieldErr = tmp
			}
		} else {
			data[k] = v
		}
	}
	return &Entry{Logger: entry.Logger, Data: data, Time: entry.Time, err: fieldErr, Context: entry.Context}
}

// WithTime overrides the time of the Entry.
func (entry *Entry) WithTime(t time.Time) *Entry {
	return &Entry{
		Logger:  entry.Logger,
		Data:    maps.Clone(entry.Data),
		Time:    t,
		Context: entry.Context,
		err:     entry.err,
	}
}

// getPackageName reduces a fully qualified function name to the package name
// There really ought to be a better way...
func getPackageName(f string) string {
	for {
		lastPeriod := strings.LastIndex(f, ".")
		lastSlash := strings.LastIndex(f, "/")
		if lastPeriod > lastSlash {
			f = f[:lastPeriod]
		} else {
			break
		}
	}

	return f
}

// getCaller retrieves the name of the first non-logrus calling function
func getCaller() *runtime.Frame {
	// cache this package's fully-qualified name
	callerInitOnce.Do(func() {
		pcs := make([]uintptr, maximumCallerDepth)
		_ = runtime.Callers(0, pcs)

		// dynamic get the package name and the minimum caller depth
		for i := range maximumCallerDepth {
			funcName := runtime.FuncForPC(pcs[i]).Name()
			if strings.Contains(funcName, "getCaller") {
				logrusPackage = getPackageName(funcName)
				break
			}
		}

		minimumCallerDepth = knownLogrusFrames
	})

	// Restrict the lookback frames to avoid runaway lookups
	pcs := make([]uintptr, maximumCallerDepth)
	depth := runtime.Callers(minimumCallerDepth, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := getPackageName(f.Function)

		// If the caller isn't part of this package, we're done
		if pkg != logrusPackage {
			return &f
		}
	}

	// if we got here, we failed to find the caller's context
	return nil
}

// HasCaller reports whether this Entry contains caller information.
//
// Caller is attached at log time if [Logger.ReportCaller] was enabled.
// In most cases, it is preferable to check whether [Entry.Caller] is nil
// directly.
func (entry Entry) HasCaller() bool {
	return entry.Caller != nil
}

func (entry *Entry) log(level Level, msg string) {
	newEntry := entry.Dup()
	logger := newEntry.Logger

	if newEntry.Time.IsZero() {
		newEntry.Time = time.Now()
	}

	newEntry.Level = level
	newEntry.Message = msg

	logger.mu.Lock()
	reportCaller := logger.ReportCaller
	bufPool := newEntry.getBufferPool()
	logger.mu.Unlock()

	if reportCaller {
		newEntry.Caller = getCaller()
	}

	// Select hooks based on the level for this log call. Hooks receive the
	// Entry and may mutate it, but that does not affect which hooks are
	// fired for this event.
	hooks := logger.hooksForLevel(level)
	newEntry.fireHooks(hooks)

	buffer := bufPool.Get()
	defer func() {
		newEntry.Buffer = nil
		buffer.Reset()
		bufPool.Put(buffer)
	}()
	buffer.Reset()
	newEntry.Buffer = buffer
	newEntry.write()
	newEntry.Buffer = nil

	// To avoid Entry#log() returning a value that only would make sense for
	// panic() to use in Entry#Panic(), we avoid the allocation by checking
	// directly here.
	if level <= PanicLevel {
		panic(newEntry)
	}
}

func (entry *Entry) getBufferPool() (pool BufferPool) {
	if entry.Logger.BufferPool != nil {
		return entry.Logger.BufferPool
	}
	return bufferPool
}

func (entry *Entry) fireHooks(hooks []Hook) {
	for _, hook := range hooks {
		if err := hook.Fire(entry); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Failed to fire hook:", err)
			return
		}
	}
}

func (entry *Entry) write() {
	// Snapshot the formatter under the lock to protect against concurrent
	// SetFormatter calls, then release the lock before formatting.
	// This avoids a deadlock when Format() triggers reentrant logging (e.g.,
	// a field's MarshalJSON calls logrus). See #1448, #1440.
	entry.Logger.mu.Lock()
	formatter := entry.Logger.Formatter
	entry.Logger.mu.Unlock()

	serialized, err := formatter.Format(entry)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to format entry:", err)
		return
	}

	// Re-acquire the lock to serialize writes to the underlying io.Writer.
	entry.Logger.mu.Lock()
	defer entry.Logger.mu.Unlock()
	if _, err := entry.Logger.Out.Write(serialized); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to write to log:", err)
	}
}

// Log logs a message at the specified level.
//
// Note: using Log with [PanicLevel] or [FatalLevel] does not trigger a panic
// or exit. For that behavior, use [Entry.Panic] or [Entry.Fatal].
func (entry *Entry) Log(level Level, args ...any) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.log(level, fmt.Sprint(args...))
	}
}

func (entry *Entry) Trace(args ...any) {
	entry.Log(TraceLevel, args...)
}

func (entry *Entry) Debug(args ...any) {
	entry.Log(DebugLevel, args...)
}

func (entry *Entry) Print(args ...any) {
	entry.Info(args...)
}

func (entry *Entry) Info(args ...any) {
	entry.Log(InfoLevel, args...)
}

func (entry *Entry) Warn(args ...any) {
	entry.Log(WarnLevel, args...)
}

func (entry *Entry) Warning(args ...any) {
	entry.Warn(args...)
}

func (entry *Entry) Error(args ...any) {
	entry.Log(ErrorLevel, args...)
}

func (entry *Entry) Fatal(args ...any) {
	entry.Log(FatalLevel, args...)
	entry.Logger.Exit(1)
}

func (entry *Entry) Panic(args ...any) {
	entry.Log(PanicLevel, args...)
}

// Entry Printf family functions

func (entry *Entry) Logf(level Level, format string, args ...any) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.Log(level, fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Tracef(format string, args ...any) {
	entry.Logf(TraceLevel, format, args...)
}

func (entry *Entry) Debugf(format string, args ...any) {
	entry.Logf(DebugLevel, format, args...)
}

func (entry *Entry) Infof(format string, args ...any) {
	entry.Logf(InfoLevel, format, args...)
}

func (entry *Entry) Printf(format string, args ...any) {
	entry.Infof(format, args...)
}

func (entry *Entry) Warnf(format string, args ...any) {
	entry.Logf(WarnLevel, format, args...)
}

func (entry *Entry) Warningf(format string, args ...any) {
	entry.Warnf(format, args...)
}

func (entry *Entry) Errorf(format string, args ...any) {
	entry.Logf(ErrorLevel, format, args...)
}

func (entry *Entry) Fatalf(format string, args ...any) {
	entry.Logf(FatalLevel, format, args...)
	entry.Logger.Exit(1)
}

func (entry *Entry) Panicf(format string, args ...any) {
	entry.Logf(PanicLevel, format, args...)
}

// Entry Println family functions

func (entry *Entry) Logln(level Level, args ...any) {
	if entry.Logger.IsLevelEnabled(level) {
		entry.Log(level, entry.sprintlnn(args...))
	}
}

func (entry *Entry) Traceln(args ...any) {
	entry.Logln(TraceLevel, args...)
}

func (entry *Entry) Debugln(args ...any) {
	entry.Logln(DebugLevel, args...)
}

func (entry *Entry) Infoln(args ...any) {
	entry.Logln(InfoLevel, args...)
}

func (entry *Entry) Println(args ...any) {
	entry.Infoln(args...)
}

func (entry *Entry) Warnln(args ...any) {
	entry.Logln(WarnLevel, args...)
}

func (entry *Entry) Warningln(args ...any) {
	entry.Warnln(args...)
}

func (entry *Entry) Errorln(args ...any) {
	entry.Logln(ErrorLevel, args...)
}

func (entry *Entry) Fatalln(args ...any) {
	entry.Logln(FatalLevel, args...)
	entry.Logger.Exit(1)
}

func (entry *Entry) Panicln(args ...any) {
	entry.Logln(PanicLevel, args...)
}

// sprintlnn => Sprint no newline. This is to get the behavior of how
// fmt.Sprintln where spaces are always added between operands, regardless of
// their type. Instead of vendoring the Sprintln implementation to spare a
// string allocation, we do the simplest thing.
func (entry *Entry) sprintlnn(args ...any) string {
	msg := fmt.Sprintln(args...)
	return msg[:len(msg)-1]
}
