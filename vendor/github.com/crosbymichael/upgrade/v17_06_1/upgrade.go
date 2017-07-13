package v17_06_1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
)

type file struct {
	name string
	x    interface{}
	buf  bytes.Buffer
	w    io.WriteCloser
}

func Upgrade(runcState, containerdConfig, containerdProcess string) error {
	files := []*file{
		&file{name: runcState, x: new(State)},
		&file{name: containerdConfig, x: new(Spec)},
		&file{name: containerdProcess, x: new(ProcessState)},
	}
	for _, f := range files {
		fd, err := os.Open(f.name)
		if err != nil {
			return err
		}
		defer fd.Close()
		// error out if any of the files have issues being decoded
		// before overwriting them, to prevent being in a mixed state.
		if err := json.NewDecoder(fd).Decode(f.x); err != nil {
			return err
		}
		// error out if any of the files have issues being encoded
		// before overwriting them, to prevent being in a mixed state.
		if err := json.NewEncoder(&f.buf).Encode(f.x); err != nil {
			return err
		}
		fi, err := fd.Stat()
		if err != nil {
			return err
		}
		f.w, err = ioutils.NewAtomicFileWriter(f.name, fi.Mode())
		if err != nil {
			return err
		}
		defer f.w.Close()
	}
	var errs []string
	for _, f := range files {
		if _, err := f.w.Write(f.buf.Bytes()); err != nil {
			errs = append(errs, fmt.Sprintf("error writing to %s: %v", f.name, err))
		}
	}
	if errs != nil {
		return fmt.Errorf(strings.Join(errs, ", "))
	}
	return nil
}
