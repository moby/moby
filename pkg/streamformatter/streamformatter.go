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

// StdoutFormatter is a streamFormatter that writes to the standard output.
type StdoutFormatter struct {
	io.Writer
	*StreamFormatter
}

func (sf *StdoutFormatter) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream(string(buf))
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}

// StderrFormatter is a streamFormatter that writes to the standard error.
type StderrFormatter struct {
	io.Writer
	*StreamFormatter
}

func (sf *StderrFormatter) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream("\033[91m" + string(buf) + "\033[0m")
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}
