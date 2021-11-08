// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel // import "go.opentelemetry.io/otel"

import (
	"log"
	"os"
	"sync"
	"sync/atomic"
)

var (
	// globalErrorHandler provides an ErrorHandler that can be used
	// throughout an OpenTelemetry instrumented project. When a user
	// specified ErrorHandler is registered (`SetErrorHandler`) all calls to
	// `Handle` and will be delegated to the registered ErrorHandler.
	globalErrorHandler = defaultErrorHandler()

	// delegateErrorHandlerOnce ensures that a user provided ErrorHandler is
	// only ever registered once.
	delegateErrorHandlerOnce sync.Once

	// Compile-time check that delegator implements ErrorHandler.
	_ ErrorHandler = (*delegator)(nil)
)

type holder struct {
	eh ErrorHandler
}

func defaultErrorHandler() *atomic.Value {
	v := &atomic.Value{}
	v.Store(holder{eh: &delegator{l: log.New(os.Stderr, "", log.LstdFlags)}})
	return v
}

// delegator logs errors if no delegate is set, otherwise they are delegated.
type delegator struct {
	delegate atomic.Value

	l *log.Logger
}

// setDelegate sets the ErrorHandler delegate.
func (h *delegator) setDelegate(d ErrorHandler) {
	// It is critical this is guarded with delegateErrorHandlerOnce, if it is
	// called again with a different concrete type it will panic.
	h.delegate.Store(d)
}

// Handle logs err if no delegate is set, otherwise it is delegated.
func (h *delegator) Handle(err error) {
	if d := h.delegate.Load(); d != nil {
		d.(ErrorHandler).Handle(err)
		return
	}
	h.l.Print(err)
}

// GetErrorHandler returns the global ErrorHandler instance.
//
// The default ErrorHandler instance returned will log all errors to STDERR
// until an override ErrorHandler is set with SetErrorHandler. All
// ErrorHandler returned prior to this will automatically forward errors to
// the set instance instead of logging.
//
// Subsequent calls to SetErrorHandler after the first will not forward errors
// to the new ErrorHandler for prior returned instances.
func GetErrorHandler() ErrorHandler {
	return globalErrorHandler.Load().(holder).eh
}

// SetErrorHandler sets the global ErrorHandler to h.
//
// The first time this is called all ErrorHandler previously returned from
// GetErrorHandler will send errors to h instead of the default logging
// ErrorHandler. Subsequent calls will set the global ErrorHandler, but not
// delegate errors to h.
func SetErrorHandler(h ErrorHandler) {
	delegateErrorHandlerOnce.Do(func() {
		current := GetErrorHandler()
		if current == h {
			return
		}
		if internalHandler, ok := current.(*delegator); ok {
			internalHandler.setDelegate(h)
		}
	})
	globalErrorHandler.Store(holder{eh: h})
}

// Handle is a convenience function for ErrorHandler().Handle(err)
func Handle(err error) {
	GetErrorHandler().Handle(err)
}
