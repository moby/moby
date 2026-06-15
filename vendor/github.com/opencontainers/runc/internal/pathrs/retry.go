// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2024-2025 SUSE LLC
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
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// Based on >50k tests running "runc run" on a 16-core system with very heavy
// rename(2) load, the single longest latency caused by -EAGAIN retries was
// ~800us (with the vast majority being closer to 400us). So, a 2ms limit
// should give more than enough headroom for any real system in practice.
const retryDeadline = 2 * time.Millisecond

// retryEAGAIN is a top-level retry loop for pathrs to try to returning
// spurious errors in most normal user cases when using openat2 (libpathrs
// itself does up to 128 retries already, but this method takes a
// wallclock-deadline approach to simply retry until a timer elapses).
func retryEAGAIN[T any](fn func() (T, error)) (T, error) {
	deadline := time.After(retryDeadline)
	for {
		v, err := fn()
		if !errors.Is(err, unix.EAGAIN) {
			return v, err
		}
		select {
		case <-deadline:
			return *new(T), fmt.Errorf("%v retry deadline exceeded: %w", retryDeadline, err)
		default:
			// retry
		}
	}
}

// retryEAGAIN2 is like retryEAGAIN except it returns two values.
func retryEAGAIN2[T1, T2 any](fn func() (T1, T2, error)) (T1, T2, error) {
	type ret struct {
		v1 T1
		v2 T2
	}
	v, err := retryEAGAIN(func() (ret, error) {
		v1, v2, err := fn()
		return ret{v1: v1, v2: v2}, err
	})
	return v.v1, v.v2, err
}
