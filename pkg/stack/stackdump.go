package stack // import "github.com/docker/docker/pkg/stack"

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const stacksLogNameTemplate = "goroutine-stacks-%s.log"

// Dump outputs the runtime stack to os.StdErr.
func Dump() {
	_ = dump(os.Stderr)
}

// DumpToFile appends the runtime stack into a file named "goroutine-stacks-<timestamp>.log"
// in dir and returns the full path to that file. If no directory name is
// provided, it outputs to os.Stderr.
func DumpToFile(dir string) (string, error) {
	var f *os.File
	if dir != "" {
		path := filepath.Join(dir, fmt.Sprintf(stacksLogNameTemplate, strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "")))
		var err error
		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return "", errors.Wrap(err, "failed to open file to write the goroutine stacks")
		}
		defer f.Close()
		defer f.Sync()
	} else {
		f = os.Stderr
	}
	return f.Name(), dump(f)
}

func dump(f *os.File) error {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	if _, err := f.Write(buf); err != nil {
		return errors.Wrap(err, "failed to write goroutine stacks")
	}
	return nil
}
