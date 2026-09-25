// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"io"
	"os"
	"time"
)

// LoggerOption is a configuration option for a Logger.
type LoggerOption interface {
	set(*Logger)
}

// CommonLabels are labels that apply to all log entries written from a Logger,
// so that you don't have to repeat them in each log entry's Labels field. If
// any of the log entries contains a (key, value) with the same key that is in
// CommonLabels, then the entry's (key, value) overrides the one in
// CommonLabels.
func CommonLabels(m map[string]string) LoggerOption { return commonLabels(m) }

type commonLabels map[string]string

func (c commonLabels) set(l *Logger) { l.commonLabels = c }

// ConcurrentWriteLimit determines how many goroutines will send log entries to the
// underlying service. The default is 1. Set ConcurrentWriteLimit to a higher value to
// increase throughput.
func ConcurrentWriteLimit(n int) LoggerOption { return concurrentWriteLimit(n) }

type concurrentWriteLimit int

func (c concurrentWriteLimit) set(l *Logger) { l.bundler.HandlerLimit = int(c) }

// DelayThreshold is the maximum amount of time that an entry should remain
// buffered in memory before a call to the logging service is triggered. Larger
// values of DelayThreshold will generally result in fewer calls to the logging
// service, while increasing the risk that log entries will be lost if the
// process crashes.
// The default is DefaultDelayThreshold.
func DelayThreshold(d time.Duration) LoggerOption { return delayThreshold(d) }

type delayThreshold time.Duration

func (d delayThreshold) set(l *Logger) { l.bundler.DelayThreshold = time.Duration(d) }

// EntryCountThreshold is the maximum number of entries that will be buffered
// in memory before a call to the logging service is triggered. Larger values
// will generally result in fewer calls to the logging service, while
// increasing both memory consumption and the risk that log entries will be
// lost if the process crashes.
// The default is DefaultEntryCountThreshold.
func EntryCountThreshold(n int) LoggerOption { return entryCountThreshold(n) }

type entryCountThreshold int

func (e entryCountThreshold) set(l *Logger) { l.bundler.BundleCountThreshold = int(e) }

// EntryByteThreshold is the maximum number of bytes of entries that will be
// buffered in memory before a call to the logging service is triggered. See
// EntryCountThreshold for a discussion of the tradeoffs involved in setting
// this option.
// The default is DefaultEntryByteThreshold.
func EntryByteThreshold(n int) LoggerOption { return entryByteThreshold(n) }

type entryByteThreshold int

func (e entryByteThreshold) set(l *Logger) { l.bundler.BundleByteThreshold = int(e) }

// EntryByteLimit is the maximum number of bytes of entries that will be sent
// in a single call to the logging service. ErrOversizedEntry is returned if an
// entry exceeds EntryByteLimit. This option limits the size of a single RPC
// payload, to account for network or service issues with large RPCs. If
// EntryByteLimit is smaller than EntryByteThreshold, the latter has no effect.
// The default is zero, meaning there is no limit.
func EntryByteLimit(n int) LoggerOption { return entryByteLimit(n) }

type entryByteLimit int

func (e entryByteLimit) set(l *Logger) { l.bundler.BundleByteLimit = int(e) }

// BufferedByteLimit is the maximum number of bytes that the Logger will keep
// in memory before returning ErrOverflow. This option limits the total memory
// consumption of the Logger (but note that each Logger has its own, separate
// limit). It is possible to reach BufferedByteLimit even if it is larger than
// EntryByteThreshold or EntryByteLimit, because calls triggered by the latter
// two options may be enqueued (and hence occupying memory) while new log
// entries are being added.
// The default is DefaultBufferedByteLimit.
func BufferedByteLimit(n int) LoggerOption { return bufferedByteLimit(n) }

type bufferedByteLimit int

func (b bufferedByteLimit) set(l *Logger) { l.bundler.BufferedByteLimit = int(b) }

// ContextFunc is a function that will be called to obtain a context.Context for the
// WriteLogEntries RPC executed in the background for calls to Logger.Log. The
// default is a function that always returns context.Background. The second return
// value of the function is a function to call after the RPC completes.
//
// The function is not used for calls to Logger.LogSync, since the caller can pass
// in the context directly.
//
// This option is EXPERIMENTAL. It may be changed or removed.
func ContextFunc(f func() (ctx context.Context, afterCall func())) LoggerOption {
	return contextFunc(f)
}

type contextFunc func() (ctx context.Context, afterCall func())

func (c contextFunc) set(l *Logger) { l.ctxFunc = c }

// SourceLocationPopulation is the flag controlling population of the source location info
// in the ingested entries. This options allows to configure automatic population of the
// SourceLocation field for all ingested entries, entries with DEBUG severity or disable it.
// Note that enabling this option can decrease execution time of Logger.Log and Logger.LogSync
// by the factor of 2 or larger.
// The default disables source location population.
//
// This option is not used when an entry is created using ToLogEntry.
func SourceLocationPopulation(f int) LoggerOption {
	return sourceLocationOption(f)
}

const (
	// DoNotPopulateSourceLocation is default for clients when WithSourceLocation is not provided
	DoNotPopulateSourceLocation = 0
	// PopulateSourceLocationForDebugEntries is set when WithSourceLocation(PopulateDebugEntries) is provided
	PopulateSourceLocationForDebugEntries = 1
	// AlwaysPopulateSourceLocation is set when WithSourceLocation(PopulateAllEntries) is provided
	AlwaysPopulateSourceLocation = 2
)

type sourceLocationOption int

func (o sourceLocationOption) set(l *Logger) {
	if o == DoNotPopulateSourceLocation || o == PopulateSourceLocationForDebugEntries || o == AlwaysPopulateSourceLocation {
		l.populateSourceLocation = int(o)
	}
}

// PartialSuccess sets the partialSuccess flag to true when ingesting a bundle of log entries.
// See https://cloud.google.com/logging/docs/reference/v2/rest/v2/entries/write#body.request_body.FIELDS.partial_success
// If not provided the partialSuccess flag is set to false.
func PartialSuccess() LoggerOption {
	return &partialSuccessOption{}
}

type partialSuccessOption struct{}

func (o *partialSuccessOption) set(l *Logger) {
	l.partialSuccess = true
}

// RedirectAsJSON instructs Logger to redirect output of calls to Log and LogSync to provided io.Writer instead of ingesting
// to Cloud Logging. Logger formats log entries following logging agent's Json format.
// See https://cloud.google.com/logging/docs/structured-logging#special-payload-fields for more info about the format.
// Use this option to delegate log ingestion to an out-of-process logging agent.
// If no writer is provided, the redirect is set to stdout.
func RedirectAsJSON(w io.Writer) LoggerOption {
	if w == nil {
		w = os.Stdout
	}
	return &redirectOutputOption{
		writer: w,
	}
}

type redirectOutputOption struct {
	writer io.Writer
}

func (o *redirectOutputOption) set(l *Logger) {
	l.redirectOutputWriter = o.writer
}
