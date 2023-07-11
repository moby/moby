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
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NewLogrusHook creates a new logrus hook
func NewLogrusHook() *LogrusHook {
	return &LogrusHook{}
}

// LogrusHook is a logrus hook which adds logrus events to active spans.
// If the span is not recording or the span context is invalid, the hook is a no-op.
type LogrusHook struct{}

// Levels returns the logrus levels that this hook is interested in.
func (h *LogrusHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log event occurs.
func (h *LogrusHook) Fire(entry *logrus.Entry) error {
	span := trace.SpanFromContext(entry.Context)
	if span == nil {
		return nil
	}

	if !span.SpanContext().IsValid() || !span.IsRecording() {
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

func logrusDataToAttrs(data logrus.Fields) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(data))
	for k, v := range data {
		attrs = append(attrs, any(k, v))
	}
	return attrs
}
