// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package deck is a Flexible Logging Framework for Go.
package deck

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// A Level is a recognized log level (Info, Error, etc). Behavior of a given level is
// backend-dependent, but they are generally used to determine how to tag, route, or
// mark-up individual log lines.
//
// Levels should not be confused with the V() attribute. V() is used to set log
// verbosity, which can be used to include or exclude logging of events, regardless
// of their associated level.
type Level uint

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

var (
	defaultDeck *Deck
)

func init() {
	defaultDeck = New()
}

// Default returns the default (global) deck.
func Default() *Deck {
	return defaultDeck
}

// Composer is the interface that groups Compose and Write methods.
type Composer interface {
	Compose(s *AttribStore) error
	Write() error
}

// Backend is the interface that identifies logging backends with NewMessage and Close methods.
type Backend interface {
	New(Level, string) Composer
	Close() error
}

// The Deck is the highest level of the logging hierarchy, consisting of one or more backends.
// All logs written to the deck get flushed to each backend. Multiple decks can be configured with
// their own sets of backends.
type Deck struct {
	backends  []Backend
	verbosity int
	mu        sync.Mutex
}

// New returns a new initialized log deck.
func New() *Deck {
	return &Deck{}
}

// Add adds a backend to the default log deck.
func Add(b Backend) {
	defaultDeck.Add(b)
}

// Add adds an additional backend to the deck.
func (d *Deck) Add(b Backend) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.backends = append(d.backends, b)
}

// SetVerbosity sets the internal verbosity level of the default deck.
func SetVerbosity(v int) {
	defaultDeck.verbosity = v
}

// SetVerbosity sets the internal verbosity level of the deck.
//
// Messages are committed if the message's own verbosity level (default 0) is
// equal to or less than the deck's configured level.
func (d *Deck) SetVerbosity(v int) {
	d.verbosity = v
}

func (d *Deck) mkLog(lvl Level, message string) *Log {
	msg := NewLog(d.verbosity)

	if len(d.backends) < 1 {
		fmt.Fprintln(os.Stderr, "WARNING: no backends configured, printing to log")
		log.Print(message)
	}

	for _, b := range d.backends {
		msg.backends = append(msg.backends, b.New(lvl, message))
	}
	return msg
}

// InfoA constructs a message in the default deck at the INFO level.
func InfoA(message ...any) *Log {
	return defaultDeck.InfoA(message...)
}

// Info immediately logs a message with no attributes to the default deck at the INFO level.
func Info(message ...any) {
	defaultDeck.InfoA(message...).With(Depth(1)).Go()
}

// InfoA constructs a message at the INFO level.
func (d *Deck) InfoA(message ...any) *Log {
	return d.mkLog(INFO, fmt.Sprint(message...))
}

// Info immediately logs a message with no attributes at the INFO level.
func (d *Deck) Info(message ...any) {
	d.InfoA(message...).With(Depth(1)).Go()
}

// InfofA constructs a message according to the format specifier in the default deck at the INFO level.
func InfofA(format string, message ...any) *Log {
	return defaultDeck.InfofA(format, message...)
}

// Infof immediately logs a message with no attributes according to the format specifier to the default deck at the INFO level.
func Infof(format string, message ...any) {
	defaultDeck.InfofA(format, message...).With(Depth(1)).Go()
}

// InfofA constructs a message according to the format specifier at the INFO level.
func (d *Deck) InfofA(format string, message ...any) *Log {
	return d.mkLog(INFO, fmt.Sprintf(format, message...))
}

// Infof immediately logs a message with no attributes according to the format specifier at the INFO level.
func (d *Deck) Infof(format string, message ...any) {
	d.InfofA(format, message...).With(Depth(1)).Go()
}

// InfolnA constructs a message with a trailing newline in the default deck at the INFO level.
func InfolnA(message ...any) *Log {
	return defaultDeck.InfolnA(message...)
}

// Infoln immediately logs a message with no attributes and with a trailing newline to the default deck at the INFO level.
func Infoln(message ...any) {
	defaultDeck.InfolnA(message...).With(Depth(1)).Go()
}

// InfolnA constructs a message with a trailing newline at the INFO level.
func (d *Deck) InfolnA(message ...any) *Log {
	return d.mkLog(INFO, fmt.Sprintln(message...))
}

// Infoln immediately logs a message with no attributes and with a trailing newline at the INFO level.
func (d *Deck) Infoln(message ...any) {
	d.InfolnA(message...).With(Depth(1)).Go()
}

// ErrorA constructs a message in the default deck at the ERROR level.
func ErrorA(message ...any) *Log {
	return defaultDeck.ErrorA(message...)
}

// Error immediately logs a message with no attributes to the default deck at the ERROR level.
func Error(message ...any) {
	defaultDeck.ErrorA(message...).With(Depth(1)).Go()
}

// ErrorA constructs a message at the ERROR level.
func (d *Deck) ErrorA(message ...any) *Log {
	return d.mkLog(ERROR, fmt.Sprint(message...))
}

// Error immediately logs a message with no attributes at the ERROR level.
func (d *Deck) Error(message ...any) {
	d.ErrorA(message...).With(Depth(1)).Go()
}

// ErrorfA constructs a message according to the format specifier in the default deck at the ERROR level.
func ErrorfA(format string, message ...any) *Log {
	return defaultDeck.ErrorfA(format, message...)
}

// Errorf immediately logs a message with no attributes according to the format specifier to the default deck at the ERROR level.
func Errorf(format string, message ...any) {
	defaultDeck.ErrorfA(format, message...).With(Depth(1)).Go()
}

// ErrorfA constructs a message according to the format specifier at the ERROR level.
func (d *Deck) ErrorfA(format string, message ...any) *Log {
	return d.mkLog(ERROR, fmt.Sprintf(format, message...))
}

// Errorf immediately logs a message with no attributes according to the format specifier at the ERROR level.
func (d *Deck) Errorf(format string, message ...any) {
	d.ErrorfA(format, message...).With(Depth(1)).Go()
}

// ErrorlnA constructs a message with a trailing newline in the default deck at the ERROR level.
func ErrorlnA(message ...any) *Log {
	return defaultDeck.ErrorlnA(message...)
}

// Errorln immediately logs a message with no attributes and with a trailing newline to the default deck at the ERROR level.
func Errorln(message ...any) {
	defaultDeck.ErrorlnA(message...).With(Depth(1)).Go()
}

// ErrorlnA constructs a message with a trailing newline at the ERROR level.
func (d *Deck) ErrorlnA(message ...any) *Log {
	return d.mkLog(ERROR, fmt.Sprintln(message...))
}

// Errorln immediately logs a message with no attributes and with a trailing newline at the ERROR level.
func (d *Deck) Errorln(message ...any) {
	d.ErrorlnA(message...).With(Depth(1)).Go()
}

// WarningA constructs a message in the default deck at the WARNING level.
func WarningA(message ...any) *Log {
	return defaultDeck.WarningA(message...)
}

// Warning immediately logs a message with no attributes to the default deck at the WARNING level.
func Warning(message ...any) {
	defaultDeck.WarningA(message...).With(Depth(1)).Go()
}

// WarningA constructs a message at the WARNING level.
func (d *Deck) WarningA(message ...any) *Log {
	return d.mkLog(WARNING, fmt.Sprint(message...))
}

// Warning immediately logs a message with no attributes at the WARNING level.
func (d *Deck) Warning(message ...any) {
	d.WarningA(message...).With(Depth(1)).Go()
}

// WarningfA constructs a message according to the format specifier in the default deck at the WARNING level.
func WarningfA(format string, message ...any) *Log {
	return defaultDeck.WarningfA(format, message...)
}

// Warningf immediately logs a message with no attributes according to the format specifier to the default deck at the WARNING level.
func Warningf(format string, message ...any) {
	defaultDeck.WarningfA(format, message...).With(Depth(1)).Go()
}

// WarningfA constructs a message according to the format specifier at the WARNING level.
func (d *Deck) WarningfA(format string, message ...any) *Log {
	return d.mkLog(WARNING, fmt.Sprintf(format, message...))
}

// Warningf immediately logs a message with no attributes according to the format specifier at the WARNING level.
func (d *Deck) Warningf(format string, message ...any) {
	d.WarningfA(format, message...).With(Depth(1)).Go()
}

// WarninglnA constructs a message with a trailing newline in the default deck at the WARNING level.
func WarninglnA(message ...any) *Log {
	return defaultDeck.WarninglnA(message...)
}

// Warningln immediately logs a message with no attributes with a trailing newline to the default deck at the WARNING level.
func Warningln(message ...any) {
	defaultDeck.WarninglnA(message...).With(Depth(1)).Go()
}

// WarninglnA constructs a message with a trailing newline at the WARNING level.
func (d *Deck) WarninglnA(message ...any) *Log {
	return d.mkLog(WARNING, fmt.Sprintln(message...))
}

// Warningln immediately logs a message with no attributes and with a trailing newline at the WARNING level.
func (d *Deck) Warningln(message ...any) {
	d.WarninglnA(message...).Go()
}

// FatalA constructs a message in the default deck at the FATAL level.
func FatalA(message ...any) *Log {
	return defaultDeck.FatalA(message...)
}

// Fatal immediately logs a message with no attributes to the default deck at the FATAL level.
func Fatal(message ...any) {
	defaultDeck.FatalA(message...).With(Depth(1)).Go()
}

// FatalA constructs a message at the FATAL level.
func (d *Deck) FatalA(message ...any) *Log {
	return d.mkLog(FATAL, fmt.Sprint(message...))
}

// Fatal immediately logs a message with no attributes at the FATAL level.
func (d *Deck) Fatal(message ...any) {
	d.FatalA(message...).With(Depth(1)).Go()
}

// FatalfA constructs a message according to the format specifier in the default deck at the FATAL level.
func FatalfA(format string, message ...any) *Log {
	return defaultDeck.FatalfA(format, message...)
}

// Fatalf immediately logs a message with no attributes according to the format specifier to the default deck at the FATAL level.
func Fatalf(format string, message ...any) {
	defaultDeck.FatalfA(format, message...).With(Depth(1)).Go()
}

// FatalfA constructs a message according to the format specifier at the FATAL level.
func (d *Deck) FatalfA(format string, message ...any) *Log {
	return d.mkLog(FATAL, fmt.Sprintf(format, message...))
}

// Fatalf immediately logs a message with no attributes according to the format specifier at the FATAL level.
func (d *Deck) Fatalf(format string, message ...any) {
	d.FatalfA(format, message...).With(Depth(1)).Go()
}

// FatallnA constructs a message with a trailing newline in the default deck at the FATAL level.
func FatallnA(message ...any) *Log {
	return defaultDeck.FatallnA(message...)
}

// Fatalln immediately logs a message with no attributes and with a trailing newline to the default deck at the FATAL level.
func Fatalln(message ...any) {
	defaultDeck.FatallnA(message...).With(Depth(1)).Go()
}

// FatallnA constructs a message with a trailing newline at the FATAL level.
func (d *Deck) FatallnA(message ...any) *Log {
	return d.mkLog(FATAL, fmt.Sprintln(message...))
}

// Fatalln immediately logs a message with no attributes and with a trailing newline at the FATAL level.
func (d *Deck) Fatalln(message ...any) {
	d.FatallnA(message...).With(Depth(1)).Go()
}

// Close closes all backends in the default deck.
func Close() {
	defaultDeck.Close()
}

// Close closes all backends in the deck.
func (d *Deck) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, b := range d.backends {
		b.Close()
	}
}

// Log is a single log event that will be flushed to all registered backends.
//
// Each log may have one or more attributes associated with it.
type Log struct {
	verbosity  int
	backends   []Composer
	attributes *AttribStore
	mu         sync.Mutex
}

// An Attrib is an attribute that can be associated with Logs.
//
// Attributes are actually functions which modify values in the AttribStore.
type Attrib func(*AttribStore)

// An AttribStore stores unique attributes associated with a given Log.
//
// The store is a string-keyed value map. Backends can interrogate the store for
// values, and use the values for their own purposes.
type AttribStore = sync.Map

// NewLog returns a new Log
func NewLog(verbosity int) *Log {
	return &Log{
		attributes: &AttribStore{},
		verbosity:  verbosity,
	}
}

// With appends one or more attributes to a Log.
//
// deck.Info("message with attributes").With(V(2), EventID(3))
func (l *Log) With(attrs ...Attrib) *Log {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, o := range attrs {
		o(l.attributes)
	}
	return l
}

// Go commits a Log to all registered backends in the deck.
func (l *Log) Go() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, o := range l.backends {
		i := 0
		if lvl, ok := l.attributes.Load("Verbosity"); ok {
			i = lvl.(int)
		}
		if i <= l.verbosity {
			o.Compose(l.attributes)
			o.Write()
		}
	}
}

// Depth is a general attribute that allows specifying log depth to backends. Depth
// may be used to modify log rendering under certain circumstances.
func Depth(d int) func(*AttribStore) {
	return func(a *AttribStore) {
		a.Store("Depth", d)
	}
}

// V is a special attribute that sets the verbosity level on a message.
//
// deck.Info("example with verbosity 2").V(2).Go()
func V(v int) func(*AttribStore) {
	return func(a *AttribStore) {
		a.Store("Verbosity", v)
	}
}
