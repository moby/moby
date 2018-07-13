// Copyright 2017, OpenCensus Authors
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

package trace

import (
	"sync"
	"time"
)

// Exporter is a type for functions that receive sampled trace spans.
//
// The ExportSpan method should be safe for concurrent use and should return
// quickly; if an Exporter takes a significant amount of time to process a
// SpanData, that work should be done on another goroutine.
//
// The SpanData should not be modified, but a pointer to it can be kept.
type Exporter interface {
	ExportSpan(s *SpanData)
}

var (
	exportersMu sync.Mutex
	exporters   map[Exporter]struct{}
)

// RegisterExporter adds to the list of Exporters that will receive sampled
// trace spans.
//
// Binaries can register exporters, libraries shouldn't register exporters.
func RegisterExporter(e Exporter) {
	exportersMu.Lock()
	if exporters == nil {
		exporters = make(map[Exporter]struct{})
	}
	exporters[e] = struct{}{}
	exportersMu.Unlock()
}

// UnregisterExporter removes from the list of Exporters the Exporter that was
// registered with the given name.
func UnregisterExporter(e Exporter) {
	exportersMu.Lock()
	delete(exporters, e)
	exportersMu.Unlock()
}

// SpanData contains all the information collected by a Span.
type SpanData struct {
	SpanContext
	ParentSpanID SpanID
	SpanKind     int
	Name         string
	StartTime    time.Time
	// The wall clock time of EndTime will be adjusted to always be offset
	// from StartTime by the duration of the span.
	EndTime time.Time
	// The values of Attributes each have type string, bool, or int64.
	Attributes    map[string]interface{}
	Annotations   []Annotation
	MessageEvents []MessageEvent
	Status
	Links           []Link
	HasRemoteParent bool
}
