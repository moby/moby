// +build !linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package fifo

import (
	"syscall"

	"github.com/pkg/errors"
)

type handle struct {
	fn  string
	dev uint64
	ino uint64
}

func getHandle(fn string) (*handle, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(fn, &stat); err != nil {
		return nil, errors.Wrapf(err, "failed to stat %v", fn)
	}

	h := &handle{
		fn:  fn,
		dev: uint64(stat.Dev),
		ino: uint64(stat.Ino),
	}

	return h, nil
}

func (h *handle) Path() (string, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(h.fn, &stat); err != nil {
		return "", errors.Wrapf(err, "path %v could not be statted", h.fn)
	}
	if uint64(stat.Dev) != h.dev || uint64(stat.Ino) != h.ino {
		return "", errors.Errorf("failed to verify handle %v/%v %v/%v for %v", stat.Dev, h.dev, stat.Ino, h.ino, h.fn)
	}
	return h.fn, nil
}

func (h *handle) Name() string {
	return h.fn
}

func (h *handle) Close() error {
	return nil
}
