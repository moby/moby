# File system notifications for Go

[![Build Status](https://goci.herokuapp.com/project/image/github.com/howeyc/fsnotify)](http://goci.me/project/github.com/howeyc/fsnotify) [![GoDoc](https://godoc.org/github.com/howeyc/fsnotify?status.png)](http://godoc.org/github.com/howeyc/fsnotify)

Cross platform, works on:
* Windows
* Linux
* BSD
* OSX

### Moving Notice

We plan to include os/fsnotify in the Go standard library with a new [API](http://goo.gl/MrYxyA). 

* Import `code.google.com/p/go.exp/fsnotify` ([GoDoc](http://godoc.org/code.google.com/p/go.exp/fsnotify)) for the latest API under development.
* Continue importing `github.com/howeyc/fsnotify` ([GoDoc](http://godoc.org/github.com/howeyc/fsnotify)) for the stable API.
* [Report Issues](https://code.google.com/p/go/issues/list?q=fsnotify) to go.exp/fsnotify after testing against `code.google.com/p/go.exp/fsnotify`
* Join [golang-dev](https://groups.google.com/forum/#!forum/golang-dev) to discuss fsnotify.
* See the [Contribution Guidelines](http://golang.org/doc/contribute.html) for Go and sign the CLA.

### Example:

```go
package main

import (
	"log"

	"github.com/howeyc/fsnotify"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)

	// Process events
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

	err = watcher.Watch("testDir")
	if err != nil {
		log.Fatal(err)
	}

	<-done

	/* ... do stuff ... */
	watcher.Close()
}
```

For each event:
* Name
* IsCreate()
* IsDelete()
* IsModify()
* IsRename()

### FAQ

**When a file is moved to another directory is it still being watched?**

No (it shouldn't be, unless you are watching where it was moved to).

**When I watch a directory, are all subdirectories watched as well?**

No, you must add watches for any directory you want to watch (a recursive watcher is in the works [#56][]).

**Do I have to watch the Error and Event channels in a separate goroutine?**

As of now, yes. Looking into making this single-thread friendly (see [#7][])

**Why am I receiving multiple events for the same file on OS X?**

Spotlight indexing on OS X can result in multiple events (see [#62][]). A temporary workaround is to add your folder(s) to the *Spotlight Privacy settings* until we have a native FSEvents implementation (see [#54][]).

**How many files can be watched at once?**

There are OS-specific limits as to how many watches can be created:
* Linux: /proc/sys/fs/inotify/max_user_watches contains the limit,
reaching this limit results in a "no space left on device" error.
* BSD / OSX: sysctl variables "kern.maxfiles" and "kern.maxfilesperproc", reaching these limits results in a "too many open files" error.


[#62]: https://github.com/howeyc/fsnotify/issues/62
[#56]: https://github.com/howeyc/fsnotify/issues/56
[#54]: https://github.com/howeyc/fsnotify/issues/54
[#7]: https://github.com/howeyc/fsnotify/issues/7

