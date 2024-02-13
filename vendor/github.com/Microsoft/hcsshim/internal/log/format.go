package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"time"
)

// TimeFormat is [time.RFC3339Nano] with nanoseconds padded using
// zeros to ensure the formatted time is always the same number of
// characters.
// Based on RFC3339NanoFixed from github.com/containerd/log
const TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}

// DurationFormat formats a [time.Duration] log entry.
//
// A nil value signals an error with the formatting.
type DurationFormat func(time.Duration) interface{}

func DurationFormatString(d time.Duration) interface{}       { return d.String() }
func DurationFormatSeconds(d time.Duration) interface{}      { return d.Seconds() }
func DurationFormatMilliseconds(d time.Duration) interface{} { return d.Milliseconds() }

// FormatIO formats net.Conn and other types that have an `Addr()` or `Name()`.
//
// See FormatEnabled for more information.
func FormatIO(ctx context.Context, v interface{}) string {
	m := make(map[string]string)
	m["type"] = reflect.TypeOf(v).String()

	switch t := v.(type) {
	case net.Conn:
		m["localAddress"] = formatAddr(t.LocalAddr())
		m["remoteAddress"] = formatAddr(t.RemoteAddr())
	case interface{ Addr() net.Addr }:
		m["address"] = formatAddr(t.Addr())
	default:
		return Format(ctx, t)
	}

	return Format(ctx, m)
}

func formatAddr(a net.Addr) string {
	return a.Network() + "://" + a.String()
}

// Format formats an object into a JSON string, without any indendtation or
// HTML escapes.
// Context is used to output a log waring if the conversion fails.
//
// This is intended primarily for `trace.StringAttribute()`
func Format(ctx context.Context, v interface{}) string {
	b, err := encode(v)
	if err != nil {
		G(ctx).WithError(err).Warning("could not format value")
		return ""
	}

	return string(b)
}

func encode(v interface{}) ([]byte, error) {
	return encodeBuffer(&bytes.Buffer{}, v)
}

func encodeBuffer(buf *bytes.Buffer, v interface{}) ([]byte, error) {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")

	if err := enc.Encode(v); err != nil {
		err = fmt.Errorf("could not marshall %T to JSON for logging: %w", v, err)
		return nil, err
	}

	// encoder.Encode appends a newline to the end
	return bytes.TrimSpace(buf.Bytes()), nil
}
