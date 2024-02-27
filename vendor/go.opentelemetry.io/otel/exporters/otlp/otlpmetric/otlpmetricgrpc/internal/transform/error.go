// Code created by gotmpl. DO NOT MODIFY.
// source: internal/shared/otlp/otlpmetric/transform/error.go.tmpl

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

package transform // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal/transform"

import (
	"errors"
	"fmt"
	"strings"

	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

var (
	errUnknownAggregation = errors.New("unknown aggregation")
	errUnknownTemporality = errors.New("unknown temporality")
)

type errMetric struct {
	m   *mpb.Metric
	err error
}

func (e errMetric) Unwrap() error {
	return e.err
}

func (e errMetric) Error() string {
	format := "invalid metric (name: %q, description: %q, unit: %q): %s"
	return fmt.Sprintf(format, e.m.Name, e.m.Description, e.m.Unit, e.err)
}

func (e errMetric) Is(target error) bool {
	return errors.Is(e.err, target)
}

// multiErr is used by the data-type transform functions to wrap multiple
// errors into a single return value. The error message will show all errors
// as a list and scope them by the datatype name that is returning them.
type multiErr struct {
	datatype string
	errs     []error
}

// errOrNil returns nil if e contains no errors, otherwise it returns e.
func (e *multiErr) errOrNil() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e
}

// append adds err to e. If err is a multiErr, its errs are flattened into e.
func (e *multiErr) append(err error) {
	// Do not use errors.As here, this should only be flattened one layer. If
	// there is a *multiErr several steps down the chain, all the errors above
	// it will be discarded if errors.As is used instead.
	switch other := err.(type) {
	case *multiErr:
		// Flatten err errors into e.
		e.errs = append(e.errs, other.errs...)
	default:
		e.errs = append(e.errs, err)
	}
}

func (e *multiErr) Error() string {
	es := make([]string, len(e.errs))
	for i, err := range e.errs {
		es[i] = fmt.Sprintf("* %s", err)
	}

	format := "%d errors occurred transforming %s:\n\t%s"
	return fmt.Sprintf(format, len(es), e.datatype, strings.Join(es, "\n\t"))
}

func (e *multiErr) Unwrap() error {
	switch len(e.errs) {
	case 0:
		return nil
	case 1:
		return e.errs[0]
	}

	// Return a multiErr without the leading error.
	cp := &multiErr{
		datatype: e.datatype,
		errs:     make([]error, len(e.errs)-1),
	}
	copy(cp.errs, e.errs[1:])
	return cp
}

func (e *multiErr) Is(target error) bool {
	if len(e.errs) == 0 {
		return false
	}
	// Check if the first error is target.
	return errors.Is(e.errs[0], target)
}
