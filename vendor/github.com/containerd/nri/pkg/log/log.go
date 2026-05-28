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

package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

var (
	log Logger = &fallbackLogger{}
)

// Logger is the interface NRI uses for logging.
type Logger interface {
	Debugf(ctx context.Context, format string, args ...interface{})
	Infof(ctx context.Context, format string, args ...interface{})
	Warnf(ctx context.Context, format string, args ...interface{})
	Errorf(ctx context.Context, format string, args ...interface{})
}

// Set the logger used by NRI.
func Set(l Logger) {
	log = l
}

// Get the logger used by NRI.
func Get() Logger {
	return log
}

// Debugf logs a formatted debug message.
func Debugf(ctx context.Context, format string, args ...interface{}) {
	log.Debugf(ctx, format, args...)
}

// Infof logs a formatted informational message.
func Infof(ctx context.Context, format string, args ...interface{}) {
	log.Infof(ctx, format, args...)
}

// Warnf logs a formatted warning message.
func Warnf(ctx context.Context, format string, args ...interface{}) {
	log.Warnf(ctx, format, args...)
}

// Errorf logs a formatted error message.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	log.Errorf(ctx, format, args...)
}

type fallbackLogger struct{}

// Debugf logs a formatted debug message.
func (f *fallbackLogger) Debugf(ctx context.Context, format string, args ...interface{}) {
	logrus.WithContext(ctx).Debugf(format, args...)
}

// Infof logs a formatted informational message.
func (f *fallbackLogger) Infof(ctx context.Context, format string, args ...interface{}) {
	logrus.WithContext(ctx).Infof(format, args...)
}

// Warnf logs a formatted warning message.
func (f *fallbackLogger) Warnf(ctx context.Context, format string, args ...interface{}) {
	logrus.WithContext(ctx).Warnf(format, args...)
}

// Errorf logs a formatted error message.
func (f *fallbackLogger) Errorf(ctx context.Context, format string, args ...interface{}) {
	logrus.WithContext(ctx).Errorf(format, args...)
}
