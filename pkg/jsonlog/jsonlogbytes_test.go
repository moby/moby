package jsonlog

import (
	"bytes"
	"regexp"
	"testing"
)

func TestJSONLogsMarshalJSONBuf(t *testing.T) {
	logs := map[*JSONLogs]string{
		&JSONLogs{Log: []byte(`"A log line with \\"`)}:           `^{\"log\":\"\\\"A log line with \\\\\\\\\\\"\",\"time\":}$`,
		&JSONLogs{Log: []byte("A log line")}:                     `^{\"log\":\"A log line\",\"time\":}$`,
		&JSONLogs{Log: []byte("A log line with \r")}:             `^{\"log\":\"A log line with \\r\",\"time\":}$`,
		&JSONLogs{Log: []byte("A log line with & < >")}:          `^{\"log\":\"A log line with \\u0026 \\u003c \\u003e\",\"time\":}$`,
		&JSONLogs{Log: []byte("A log line with utf8 : 🚀 ψ ω β")}: `^{\"log\":\"A log line with utf8 : 🚀 ψ ω β\",\"time\":}$`,
		&JSONLogs{Stream: "stdout"}:                              `^{\"stream\":\"stdout\",\"time\":}$`,
		&JSONLogs{Stream: "stdout", Log: []byte("A log line")}:   `^{\"log\":\"A log line\",\"stream\":\"stdout\",\"time\":}$`,
		&JSONLogs{Created: "time"}:                               `^{\"time\":time}$`,
		&JSONLogs{}:                                              `^{\"time\":}$`,
		// These ones are a little weird
		&JSONLogs{Log: []byte("\u2028 \u2029")}: `^{\"log\":\"\\u2028 \\u2029\",\"time\":}$`,
		&JSONLogs{Log: []byte{0xaF}}:            `^{\"log\":\"\\ufffd\",\"time\":}$`,
		&JSONLogs{Log: []byte{0x7F}}:            `^{\"log\":\"\x7f\",\"time\":}$`,
		// with raw attributes
		&JSONLogs{Log: []byte("A log line"), RawAttrs: []byte(`{"hello":"world","value":1234}`)}: `^{\"log\":\"A log line\",\"attrs\":{\"hello\":\"world\",\"value\":1234},\"time\":}$`,
	}
	for jsonLog, expression := range logs {
		var buf bytes.Buffer
		if err := jsonLog.MarshalJSONBuf(&buf); err != nil {
			t.Fatal(err)
		}
		res := buf.String()
		t.Logf("Result of WriteLog: %q", res)
		logRe := regexp.MustCompile(expression)
		if !logRe.MatchString(res) {
			t.Fatalf("Log line not in expected format [%v]: %q", expression, res)
		}
	}
}
