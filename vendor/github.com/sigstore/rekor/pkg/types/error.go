//
// Copyright 2021 The Sigstore Authors.
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

package types

// ValidationError indicates that there is an issue with the content in the HTTP Request that
// should result in an HTTP 400 Bad Request error being returned to the client
//
// Deprecated: use InputValidationError instead to take advantage of Go's error wrapping
type ValidationError error

// InputValidationError indicates that there is an issue with the content in the HTTP Request that
// should result in an HTTP 400 Bad Request error being returned to the client
type InputValidationError struct {
	Err error
}

func (v *InputValidationError) Error() string { return v.Err.Error() }
func (v *InputValidationError) Unwrap() error { return v.Err }
