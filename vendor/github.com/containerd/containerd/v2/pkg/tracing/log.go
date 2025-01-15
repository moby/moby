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

package tracing

import (
	"github.com/containerd/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// allLevels is the equivalent to [logrus.AllLevels].
//
// [logrus.AllLevels]: https://github.com/sirupsen/logrus/blob/v1.9.3/logrus.go#L80-L89
var allLevels = []log.Level{
	log.PanicLevel,
	log.FatalLevel,
	log.ErrorLevel,
	log.WarnLevel,
	log.InfoLevel,
	log.DebugLevel,
	log.TraceLevel,
}

// NewLogrusHook creates a new logrus hook
func NewLogrusHook() *LogrusHook {
	return &LogrusHook{}
}

// LogrusHook is a [logrus.Hook] which adds logrus events to active spans.
// If the span is not recording or the span context is invalid, the hook
// is a no-op.
//
// [logrus.Hook]: https://github.com/sirupsen/logrus/blob/v1.9.3/hooks.go#L3-L11
type LogrusHook struct{}

// Levels returns the logrus levels that this hook is interested in.
func (h *LogrusHook) Levels() []log.Level {
	return allLevels
}

// Fire is called when a log event occurs.
func (h *LogrusHook) Fire(entry *log.Entry) error {
	span := trace.SpanFromContext(entry.Context)
	if span == nil {
		return nil
	}

	if !span.IsRecording() || !span.SpanContext().IsValid() {
		return nil
	}

	span.AddEvent(
		entry.Message,
		trace.WithAttributes(logrusDataToAttrs(entry.Data)...),
		trace.WithAttributes(attribute.String("level", entry.Level.String())),
		trace.WithTimestamp(entry.Time),
	)

	return nil
}

func logrusDataToAttrs(data map[string]any) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(data))
	for k, v := range data {
		attrs = append(attrs, keyValue(k, v))
	}
	return attrs
}
