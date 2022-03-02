package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/jsonfilelog/jsonlog"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/sirupsen/logrus"
)

const maxJSONDecodeRetry = 20000

// ReadLogs implements the logger's LogReader interface for the logs
// created by this driver.
func (l *JSONFileLogger) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	return l.writer.ReadLogs(config)
}

func decodeLogLine(dec *json.Decoder, l *jsonlog.JSONLog) (*logger.Message, error) {
	l.Reset()
	if err := dec.Decode(l); err != nil {
		return nil, err
	}

	var attrs []backend.LogAttr
	if len(l.Attrs) != 0 {
		attrs = make([]backend.LogAttr, 0, len(l.Attrs))
		for k, v := range l.Attrs {
			attrs = append(attrs, backend.LogAttr{Key: k, Value: v})
		}
	}
	msg := &logger.Message{
		Source:    l.Stream,
		Timestamp: l.Created,
		Line:      []byte(l.Log),
		Attrs:     attrs,
	}
	return msg, nil
}

type decoder struct {
	rdr      io.Reader
	dec      *json.Decoder
	jl       *jsonlog.JSONLog
	maxRetry int
}

func (d *decoder) Reset(rdr io.Reader) {
	d.rdr = rdr
	d.dec = nil
	if d.jl != nil {
		d.jl.Reset()
	}
}

func (d *decoder) Close() {
	d.dec = nil
	d.rdr = nil
	d.jl = nil
}

func (d *decoder) Decode() (msg *logger.Message, err error) {
	if d.dec == nil {
		d.dec = json.NewDecoder(d.rdr)
	}
	if d.jl == nil {
		d.jl = &jsonlog.JSONLog{}
	}
	if d.maxRetry == 0 {
		// We aren't using maxJSONDecodeRetry directly so we can give a custom value for testing.
		d.maxRetry = maxJSONDecodeRetry
	}
	for retries := 0; retries < d.maxRetry; retries++ {
		msg, err = decodeLogLine(d.dec, d.jl)
		if err == nil || err == io.EOF {
			break
		}

		logrus.WithError(err).WithField("retries", retries).Warn("got error while decoding json")
		// try again, could be due to a an incomplete json object as we read
		if _, ok := err.(*json.SyntaxError); ok {
			d.dec = json.NewDecoder(d.rdr)
			continue
		}

		// io.ErrUnexpectedEOF is returned from json.Decoder when there is
		// remaining data in the parser's buffer while an io.EOF occurs.
		// If the json logger writes a partial json log entry to the disk
		// while at the same time the decoder tries to decode it, the race condition happens.
		if err == io.ErrUnexpectedEOF {
			d.rdr = combineReaders(d.dec.Buffered(), d.rdr)
			d.dec = json.NewDecoder(d.rdr)
			continue
		}
	}
	return msg, err
}

func combineReaders(pre, rdr io.Reader) io.Reader {
	return &combinedReader{pre: pre, rdr: rdr}
}

// combinedReader is a reader which is like `io.MultiReader` where except it does not cache a full EOF.
// Once `io.MultiReader` returns EOF, it is always EOF.
//
// For this usecase we have an underlying reader which is a file which may reach EOF but have more data written to it later.
// As such, io.MultiReader does not work for us.
type combinedReader struct {
	pre io.Reader
	rdr io.Reader
}

func (r *combinedReader) Read(p []byte) (int, error) {
	var read int
	if r.pre != nil {
		n, err := r.pre.Read(p)
		if err != nil {
			if err != io.EOF {
				return n, err
			}
			r.pre = nil
		}
		read = n
	}

	if read < len(p) {
		n, err := r.rdr.Read(p[read:])
		if n > 0 {
			read += n
		}
		if err != nil {
			return read, err
		}
	}

	return read, nil
}

// decodeFunc is used to create a decoder for the log file reader
func decodeFunc(rdr io.Reader) loggerutils.Decoder {
	return &decoder{
		rdr: rdr,
		dec: nil,
		jl:  nil,
	}
}

func getTailReader(ctx context.Context, r loggerutils.SizeReaderAt, req int) (io.Reader, int, error) {
	return tailfile.NewTailReader(ctx, r, req)
}
