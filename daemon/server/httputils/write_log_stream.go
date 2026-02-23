package httputils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"

	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/daemon/internal/stdcopymux"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/pkg/ioutils"
)

// rfc3339NanoFixed is time.RFC3339Nano with nanoseconds padded using zeros to
// ensure the formatted time isalways the same number of characters.
const rfc3339NanoFixed = "2006-01-02T15:04:05.000000000Z07:00"

// WriteLogStream writes an encoded byte stream of log messages from the
// messages channel, multiplexing them with a stdcopy.Writer if mux is true
func WriteLogStream(_ context.Context, w http.ResponseWriter, msgs <-chan *backend.LogMessage, config *backend.ContainerLogsOptions, mux bool) {
	// See https://github.com/moby/moby/issues/47448
	// Trigger headers to be written immediately.
	w.WriteHeader(http.StatusOK)

	wf := ioutils.NewWriteFlusher(w)
	defer wf.Close()

	wf.Flush()

	outStream := io.Writer(wf)
	errStream := outStream
	sysErrStream := errStream
	if mux {
		sysErrStream = stdcopymux.NewStdWriter(outStream, stdcopy.Systemerr)
		errStream = stdcopymux.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopymux.NewStdWriter(outStream, stdcopy.Stdout)
	}

	for {
		msg, ok := <-msgs
		if !ok {
			return
		}
		// check if the message contains an error. if so, write that error
		// and exit
		if msg.Err != nil {
			fmt.Fprintf(sysErrStream, "Error grabbing logs: %v\n", msg.Err)
			continue
		}
		logLine := msg.Line
		if config.Details {
			logLine = append(attrsByteSlice(msg.Attrs), ' ')
			logLine = append(logLine, msg.Line...)
		}
		if config.Timestamps {
			logLine = append([]byte(msg.Timestamp.Format(rfc3339NanoFixed)+" "), logLine...)
		}
		if msg.Source == "stdout" && config.ShowStdout {
			_, _ = outStream.Write(logLine)
		}
		if msg.Source == "stderr" && config.ShowStderr {
			_, _ = errStream.Write(logLine)
		}
	}
}

type byKey []backend.LogAttr

func (b byKey) Len() int           { return len(b) }
func (b byKey) Less(i, j int) bool { return b[i].Key < b[j].Key }
func (b byKey) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func attrsByteSlice(a []backend.LogAttr) []byte {
	// Note this sorts "a" in-place. That is fine here - nothing else is
	// going to use Attrs or care about the order.
	sort.Sort(byKey(a))

	var ret []byte
	for i, pair := range a {
		k, v := url.QueryEscape(pair.Key), url.QueryEscape(pair.Value)
		ret = append(ret, []byte(k)...)
		ret = append(ret, '=')
		ret = append(ret, []byte(v)...)
		if i != len(a)-1 {
			ret = append(ret, ',')
		}
	}
	return ret
}
