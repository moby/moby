// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9,!solaris

// Package fsnotify provides a platform-independent interface for file system notifications.
package fsnotify

import "fmt"

// Event represents a single file system notification.
type Event struct {
	Name string // Relative path to the file or directory.
	Op   Op     // File operation that triggered the event.
}

// Op describes a set of file operations.
type Op uint32

// These are the generalized file operations that can trigger a notification.
const (
	Create Op = 1 << iota
	Write
	Remove
	Rename
	Chmod
)

// String returns a string representation of the event in the form
// "file: REMOVE|WRITE|..."
func (e Event) String() string {
	events := ""

	if e.Op&Create == Create {
		events += "|CREATE"
	}
	if e.Op&Remove == Remove {
		events += "|REMOVE"
	}
	if e.Op&Write == Write {
		events += "|WRITE"
	}
	if e.Op&Rename == Rename {
		events += "|RENAME"
	}
	if e.Op&Chmod == Chmod {
		events += "|CHMOD"
	}

	if len(events) > 0 {
		events = events[1:]
	}

	return fmt.Sprintf("%q: %s", e.Name, events)
}
