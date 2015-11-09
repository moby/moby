// Package streamformatter provides helper functions to format a stream.
package streamformatter

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
)

// StreamFormatter formats a stream, optionally using JSON.
type StreamFormatter struct {
	json bool
}

// NewStreamFormatter returns a simple StreamFormatter
func NewStreamFormatter() *StreamFormatter {
	return &StreamFormatter{}
}

// NewJSONStreamFormatter returns a StreamFormatter configured to stream json
func NewJSONStreamFormatter() *StreamFormatter {
	return &StreamFormatter{true}
}

const streamNewline = "\r\n"

var streamNewlineBytes = []byte(streamNewline)

// FormatStream formats the specified stream.
func (sf *StreamFormatter) FormatStream(str string) []byte {
	if sf.json {
		b, err := json.Marshal(&jsonmessage.JSONMessage{Stream: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return append(b, streamNewlineBytes...)
	}
	return []byte(str + "\r")
}

// FormatStatus formats the specified objects according to the specified format (and id).
func (sf *StreamFormatter) FormatStatus(id, format string, a ...interface{}) []byte {
	str := fmt.Sprintf(format, a...)
	if sf.json {
		b, err := json.Marshal(&jsonmessage.JSONMessage{ID: id, Status: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return append(b, streamNewlineBytes...)
	}
	return []byte(str + streamNewline)
}

// FormatError formats the specifed error.
func (sf *StreamFormatter) FormatError(err error) []byte {
	if sf.json {
		jsonError, ok := err.(*jsonmessage.JSONError)
		if !ok {
			jsonError = &jsonmessage.JSONError{Message: err.Error()}
		}
		if b, err := json.Marshal(&jsonmessage.JSONMessage{Error: jsonError, ErrorMessage: err.Error()}); err == nil {
			return append(b, streamNewlineBytes...)
		}
		return []byte("{\"error\":\"format error\"}" + streamNewline)
	}
	return []byte("Error: " + err.Error() + streamNewline)
}

// FormatProgress formats the progress information for a specified action.
func (sf *StreamFormatter) FormatProgress(id, action string, progress *jsonmessage.JSONProgress) []byte {
	if progress == nil {
		progress = &jsonmessage.JSONProgress{}
	}
	if sf.json {
		b, err := json.Marshal(&jsonmessage.JSONMessage{
			Status:          action,
			ProgressMessage: progress.String(),
			Progress:        progress,
			ID:              id,
		})
		if err != nil {
			return nil
		}
		return b
	}
	endl := "\r"
	if progress.String() == "" {
		endl += "\n"
	}
	return []byte(action + " " + progress.String() + endl)
}

// IOFormattedWriter holds a writer and a formatter to write information
type IOFormattedWriter struct {
	io.Writer
	*StreamFormatter
}

// Write writes a byte buffer stream formatted.
func (sf *IOFormattedWriter) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream(string(buf))
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}

// WriteError writes an error formatted.
func (sf *IOFormattedWriter) WriteError(err error) (int, error) {
	buf := sf.StreamFormatter.FormatError(err)
	return sf.Writer.Write(buf)
}

// WriteStatus writes a status formatted.
func (sf *IOFormattedWriter) WriteStatus(id, format string, a ...interface{}) (int, error) {
	buf := sf.StreamFormatter.FormatStatus(id, format, a...)
	return sf.Writer.Write(buf)
}

// WriteProgress writes progress status formatted.
func (sf *IOFormattedWriter) WriteProgress(id, action string, progress *jsonmessage.JSONProgress) (int, error) {
	fmtMessage := sf.StreamFormatter.FormatProgress(id, action, progress)
	return sf.Writer.Write(fmtMessage)
}

// StdoutFormattedWriter is a streamFormatter that writes to the standard output.
type StdoutFormattedWriter struct {
	*IOFormattedWriter
}

// NewStdoutFormattedWriter initializes an StdoutFormattedWriter with a plain stream formatter.
func NewStdoutFormattedWriter(w io.Writer) *StdoutFormattedWriter {
	return NewStdoutCustomFormattedWriter(w, NewStreamFormatter())
}

// NewStdoutJSONFormattedWriter initializes an StdoutFormattedWriter with a JSON stream formatter.
func NewStdoutJSONFormattedWriter(w io.Writer) *StdoutFormattedWriter {
	return NewStdoutCustomFormattedWriter(w, NewJSONStreamFormatter())
}

// NewStdoutCustomFormattedWriter initializes an StdoutFormattedWriter with a writer and a custom formatter.
func NewStdoutCustomFormattedWriter(w io.Writer, sf *StreamFormatter) *StdoutFormattedWriter {
	return &StdoutFormattedWriter{&IOFormattedWriter{Writer: w, StreamFormatter: sf}}
}

// StderrFormattedWriter is a streamFormatter that writes to the standard error.
type StderrFormattedWriter struct {
	*IOFormattedWriter
}

// NewStderrJSONFormattedWriter initializes an StderrFormattedWriter with a JSON stream formatter.
func NewStderrJSONFormattedWriter(w io.Writer) *StderrFormattedWriter {
	return &StderrFormattedWriter{&IOFormattedWriter{Writer: w, StreamFormatter: NewJSONStreamFormatter()}}
}

// Write writes a byte buffer stream formatted.
func (sf *StderrFormattedWriter) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream("\033[91m" + string(buf) + "\033[0m")
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}
