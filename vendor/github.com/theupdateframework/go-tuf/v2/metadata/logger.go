// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package metadata

var log Logger = DiscardLogger{}

// Logger partially implements the go-log/logr's interface:
// https://github.com/go-logr/logr/blob/master/logr.go
type Logger interface {
	// Info logs a non-error message with key/value pairs
	Info(msg string, kv ...any)
	// Error logs an error with a given message and key/value pairs.
	Error(err error, msg string, kv ...any)
}

type DiscardLogger struct{}

func (d DiscardLogger) Info(msg string, kv ...any) {
}

func (d DiscardLogger) Error(err error, msg string, kv ...any) {
}

func SetLogger(logger Logger) {
	log = logger
}

func GetLogger() Logger {
	return log
}
