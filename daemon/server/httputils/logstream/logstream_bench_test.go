package logstream_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils/logstream"
)

type discardResponseWriter struct{}

func (discardResponseWriter) Header() http.Header        { return nil }
func (discardResponseWriter) WriteHeader(statusCode int) {}
func (discardResponseWriter) Flush()                     {}
func (discardResponseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func BenchmarkWrite(b *testing.B) {
	const line = "hello world\n"

	for _, tc := range []struct {
		name   string
		config backend.ContainerLogsOptions
	}{
		{
			name: "Plain",
			config: backend.ContainerLogsOptions{
				ShowStdout: true,
			},
		},
		{
			name: "Timestamps",
			config: backend.ContainerLogsOptions{
				ShowStdout: true,
				Timestamps: true,
			},
		},
		{
			name: "Details",
			config: backend.ContainerLogsOptions{
				ShowStdout: true,
				Details:    true,
			},
		},
		{
			name: "TimestampsAndDetails",
			config: backend.ContainerLogsOptions{
				ShowStdout: true,
				Timestamps: true,
				Details:    true,
			},
		},
		{
			name: "DiscardedWithTimestampsAndDetails",
			config: backend.ContainerLogsOptions{
				ShowStdout: false,
				Timestamps: true,
				Details:    true,
			},
		},
		{
			name: "Discarded",
			config: backend.ContainerLogsOptions{
				ShowStdout: false,
			},
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			msg := &backend.LogMessage{
				Source:    "stdout",
				Line:      []byte(line),
				Timestamp: time.Date(2026, 9, 2, 12, 34, 56, 123456789, time.UTC),
				Attrs: []backend.LogAttr{
					{Key: "container", Value: "example"},
					{Key: "foo", Value: "bar"},
				},
			}

			msgs := make(chan *backend.LogMessage)
			go func() {
				for range b.N {
					msgs <- msg
				}
				close(msgs)
			}()

			b.ReportAllocs()
			b.SetBytes(int64(len(line)))
			b.ResetTimer()

			logstream.Write(b.Context(), discardResponseWriter{}, msgs, &tc.config, false)
		})
	}
}
