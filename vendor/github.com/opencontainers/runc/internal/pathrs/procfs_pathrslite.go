// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2025 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pathrs

import (
	"fmt"
	"os"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
)

func procOpenReopen(openFn func(subpath string) (*os.File, error), subpath string, flags int) (*os.File, error) {
	handle, err := retryEAGAIN(func() (*os.File, error) {
		return openFn(subpath)
	})
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	f, err := Reopen(handle, flags)
	if err != nil {
		return nil, fmt.Errorf("reopen %s: %w", handle.Name(), err)
	}
	return f, nil
}

// ProcSelfOpen is a wrapper around [procfs.Handle.OpenSelf] and
// [pathrs.Reopen], to let you one-shot open a procfs file with the given
// flags.
func ProcSelfOpen(subpath string, flags int) (*os.File, error) {
	proc, err := retryEAGAIN(procfs.OpenProcRoot)
	if err != nil {
		return nil, err
	}
	defer proc.Close()
	return procOpenReopen(proc.OpenSelf, subpath, flags)
}

// ProcPidOpen is a wrapper around [procfs.Handle.OpenPid] and [pathrs.Reopen],
// to let you one-shot open a procfs file with the given flags.
func ProcPidOpen(pid int, subpath string, flags int) (*os.File, error) {
	proc, err := retryEAGAIN(procfs.OpenProcRoot)
	if err != nil {
		return nil, err
	}
	defer proc.Close()
	return procOpenReopen(func(subpath string) (*os.File, error) {
		return proc.OpenPid(pid, subpath)
	}, subpath, flags)
}

// ProcThreadSelfOpen is a wrapper around [procfs.Handle.OpenThreadSelf] and
// [pathrs.Reopen], to let you one-shot open a procfs file with the given
// flags. The returned [procfs.ProcThreadSelfCloser] needs the same handling as
// when using pathrs-lite.
func ProcThreadSelfOpen(subpath string, flags int) (_ *os.File, _ procfs.ProcThreadSelfCloser, Err error) {
	proc, err := retryEAGAIN(procfs.OpenProcRoot)
	if err != nil {
		return nil, nil, err
	}
	defer proc.Close()

	handle, closer, err := retryEAGAIN2(func() (*os.File, procfs.ProcThreadSelfCloser, error) {
		return proc.OpenThreadSelf(subpath)
	})
	if err != nil {
		return nil, nil, err
	}
	if closer != nil {
		defer func() {
			if Err != nil {
				closer()
			}
		}()
	}
	defer handle.Close()

	f, err := Reopen(handle, flags)
	if err != nil {
		return nil, nil, fmt.Errorf("reopen %s: %w", handle.Name(), err)
	}
	return f, closer, nil
}

// Reopen is a wrapper around pathrs.Reopen.
func Reopen(file *os.File, flags int) (*os.File, error) {
	return retryEAGAIN(func() (*os.File, error) {
		return pathrs.Reopen(file, flags)
	})
}
