// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"strings"

	"cloud.google.com/go/logging/internal"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

const diagnosticLogID = "diagnostic-log"

// instrumentationPayload defines telemetry log entry payload for capturing instrumentation info
type instrumentationPayload struct {
	InstrumentationSource []map[string]string `json:"instrumentation_source"`
	Runtime               string              `json:"runtime,omitempty"`
}

var (
	instrumentationInfo = &instrumentationPayload{
		InstrumentationSource: []map[string]string{
			{
				"name":    "go",
				"version": internal.Version,
			},
		},
		Runtime: internal.VersionGo(),
	}
)

// instrumentLogs appends log entry with library instrumentation info to the
// list of log entries on the first function's call.
func (l *Logger) instrumentLogs(entries []*logpb.LogEntry) ([]*logpb.LogEntry, bool) {
	var instrumentationAdded bool

	internal.InstrumentOnce.Do(func() {
		ie, err := l.instrumentationEntry()
		if err != nil {
			// do not retry instrumenting logs if failed creating instrumentation entry
			return
		}
		// populate LogName only when  directly ingesting entries
		if l.redirectOutputWriter == nil {
			ie.LogName = internal.LogPath(l.client.parent, diagnosticLogID)
		}
		entries = append(entries, ie)
		instrumentationAdded = true
	})
	return entries, instrumentationAdded
}

func (l *Logger) instrumentationEntry() (*logpb.LogEntry, error) {
	ent := Entry{
		Payload: map[string]*instrumentationPayload{
			"logging.googleapis.com/diagnostic": instrumentationInfo,
		},
	}
	// pass nil for Logger and 0 for skip levels to ignore auto-population
	return toLogEntryInternal(ent, nil, l.client.parent, 0)
}

// hasInstrumentation returns true if any of the log entries has diagnostic LogId
func hasInstrumentation(entries []*logpb.LogEntry) bool {
	for _, ent := range entries {
		if strings.HasSuffix(ent.LogName, diagnosticLogID) {
			return true
		}
	}
	return false
}
