/*
   Copyright © 2022 The CDI Authors

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

package multierror

import (
	"strings"
)

// New combines several errors into a single error. Parameters that are nil are
// ignored. If no errors are passed in or all parameters are nil, then the
// result is also nil.
func New(errors ...error) error {
	// Filter out nil entries.
	numErrors := 0
	for _, err := range errors {
		if err != nil {
			errors[numErrors] = err
			numErrors++
		}
	}
	if numErrors == 0 {
		return nil
	}
	return multiError(errors[0:numErrors])
}

// multiError is the underlying implementation used by New.
//
// Beware that a null multiError is not the same as a nil error.
type multiError []error

// multiError returns all individual error strings concatenated with "\n"
func (e multiError) Error() string {
	var builder strings.Builder
	for i, err := range e {
		if i > 0 {
			_, _ = builder.WriteString("\n")
		}
		_, _ = builder.WriteString(err.Error())
	}
	return builder.String()
}

// Append returns a new multi error all errors concatenated. Errors that are
// multi errors get flattened, nil is ignored.
func Append(err error, errors ...error) error {
	var result multiError
	if m, ok := err.(multiError); ok {
		result = m
	} else if err != nil {
		result = append(result, err)
	}

	for _, e := range errors {
		if e == nil {
			continue
		}
		if m, ok := e.(multiError); ok {
			result = append(result, m...)
		} else {
			result = append(result, e)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
