// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjson

import (
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// MarshalExportTraceServiceResponse encodes an ExportTraceServiceResponse as JSON Protobuf encoded bytes.
func MarshalExportTraceServiceResponse(resp *coltracepb.ExportTraceServiceResponse) ([]byte, error) {
	return protojson.Marshal(resp)
}

// UnmarshalExportTraceServiceResponse decodes JSON Protobuf encoded payload into an ExportTraceServiceResponse.
func UnmarshalExportTraceServiceResponse(data []byte, resp *coltracepb.ExportTraceServiceResponse) error {
	// ignore message fields with unknown names per OTLP specs.
	var unmarshaler protojson.UnmarshalOptions
	unmarshaler.DiscardUnknown = true
	return unmarshaler.Unmarshal(data, resp)
}
