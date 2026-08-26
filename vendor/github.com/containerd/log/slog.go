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
	"io"
	"log/slog"
	"sync"

	"github.com/sirupsen/logrus"
)

// slogOut is used to set the slog logger when setting output format.
var slogOut io.Writer

// slogLevel is used to control the slog handler's level when slog output is active.
var slogLevel = &slog.LevelVar{}

// slogOnce guards UseSlog so repeated calls do not stack up hooks or
// reset slogOut to the discard writer installed on the first call.
var slogOnce sync.Once

func UseSlog() {
	slogOnce.Do(func() {
		L.Logger.SetNoLock()
		L.Logger.AddHook(slogHook{})
		slogOut = L.Logger.Out
		L.Logger.SetOutput(io.Discard)
		slogLevel.Set(logrusToSlogLevel(L.Logger.GetLevel()))
	})
}

type slogHook struct{}

func (hook slogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func logrusToSlogLevel(l logrus.Level) slog.Level {
	switch l {
	case logrus.PanicLevel:
		return slog.LevelError + 4
	case logrus.FatalLevel:
		return slog.LevelError + 2
	case logrus.ErrorLevel:
		return slog.LevelError
	case logrus.WarnLevel:
		return slog.LevelWarn
	case logrus.DebugLevel:
		return slog.LevelDebug
	case logrus.TraceLevel:
		return slog.LevelDebug - 4
	default:
		return slog.LevelInfo
	}
}

func (hook slogHook) Fire(entry *logrus.Entry) error {
	level := logrusToSlogLevel(entry.Level)

	handler := slog.Default().Handler()

	ctx := entry.Context
	if ctx == nil {
		ctx = context.Background()
	}

	if !handler.Enabled(ctx, level) {
		return nil
	}

	record := slog.NewRecord(entry.Time, level, entry.Message, 0)

	// Convert logrus fields to slog attributes.
	for k, v := range entry.Data {
		record.AddAttrs(slog.Any(k, v))
	}

	return handler.Handle(ctx, record)
}
