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
	"encoding/json"
	"fmt"

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

func any(k string, v interface{}) attribute.KeyValue {
	if v == nil {
		return attribute.String(k, "<nil>")
	}

	switch typed := v.(type) {
	case bool:
		return attribute.Bool(k, typed)
	case []bool:
		return attribute.BoolSlice(k, typed)
	case int:
		return attribute.Int(k, typed)
	case []int:
		return attribute.IntSlice(k, typed)
	case int8:
		return attribute.Int(k, int(typed))
	case []int8:
		ls := make([]int, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int(i))
		}
		return attribute.IntSlice(k, ls)
	case int16:
		return attribute.Int(k, int(typed))
	case []int16:
		ls := make([]int, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int(i))
		}
		return attribute.IntSlice(k, ls)
	case int32:
		return attribute.Int64(k, int64(typed))
	case []int32:
		ls := make([]int64, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int64(i))
		}
		return attribute.Int64Slice(k, ls)
	case int64:
		return attribute.Int64(k, typed)
	case []int64:
		return attribute.Int64Slice(k, typed)
	case float64:
		return attribute.Float64(k, typed)
	case []float64:
		return attribute.Float64Slice(k, typed)
	case string:
		return attribute.String(k, typed)
	case []string:
		return attribute.StringSlice(k, typed)
	}

	if stringer, ok := v.(fmt.Stringer); ok {
		return attribute.String(k, stringer.String())
	}
	if b, err := json.Marshal(v); b != nil && err == nil {
		return attribute.String(k, string(b))
	}
	return attribute.String(k, fmt.Sprintf("%v", v))
}
