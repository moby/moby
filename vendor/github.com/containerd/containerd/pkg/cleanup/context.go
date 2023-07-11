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

// Package providing utilies to help cleanup
package cleanup

import (
	"context"
	"time"
)

type clearCancel struct {
	context.Context
}

func (cc clearCancel) Deadline() (deadline time.Time, ok bool) {
	return
}

func (cc clearCancel) Done() <-chan struct{} {
	return nil
}

func (cc clearCancel) Err() error {
	return nil
}

// Background creates a new context which clears out the parent errors
func Background(ctx context.Context) context.Context {
	return clearCancel{ctx}
}

// Do runs the provided function with a context in which the
// errors are cleared out and will timeout after 10 seconds.
func Do(ctx context.Context, do func(context.Context)) {
	ctx, cancel := context.WithTimeout(clearCancel{ctx}, 10*time.Second)
	do(ctx)
	cancel()
}
