// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fsnotify_test

import (
	"log"

	"github.com/howeyc/fsnotify"
)

func ExampleNewWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				log.Println("event:", ev)
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Watch("/tmp/foo")
	if err != nil {
		log.Fatal(err)
	}
}
