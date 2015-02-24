package utils

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/pkg/progressreader"
	"io"
)

type StreamFormatter struct {
	json bool
}

func NewStreamFormatter(json bool) *StreamFormatter {
	return &StreamFormatter{json}
}

const streamNewline = "\r\n"

var streamNewlineBytes = []byte(streamNewline)

func (sf *StreamFormatter) FormatStream(str string) []byte {
	if sf.json {
		b, err := json.Marshal(&JSONMessage{Stream: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return append(b, streamNewlineBytes...)
	}
	return []byte(str + "\r")
}

func (sf *StreamFormatter) FormatStatus(id, format string, a ...interface{}) []byte {
	str := fmt.Sprintf(format, a...)
	if sf.json {
		b, err := json.Marshal(&JSONMessage{ID: id, Status: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return append(b, streamNewlineBytes...)
	}
	return []byte(str + streamNewline)
}

func (sf *StreamFormatter) FormatError(err error) []byte {
	if sf.json {
		jsonError, ok := err.(*JSONError)
		if !ok {
			jsonError = &JSONError{Message: err.Error()}
		}
		if b, err := json.Marshal(&JSONMessage{Error: jsonError, ErrorMessage: err.Error()}); err == nil {
			return append(b, streamNewlineBytes...)
		}
		return []byte("{\"error\":\"format error\"}" + streamNewline)
	}
	return []byte("Error: " + err.Error() + streamNewline)
}
func (sf *StreamFormatter) FormatProg(id, action string, p interface{}) []byte {
	switch progress := p.(type) {
	case *JSONProgress:
		return sf.FormatProgress(id, action, progress)
	case progressreader.PR_JSONProgress:
		return sf.FormatProgress(id, action, &JSONProgress{Current: progress.GetCurrent(), Total: progress.GetTotal()})
	}
	return nil
}
func (sf *StreamFormatter) FormatProgress(id, action string, progress *JSONProgress) []byte {
	if progress == nil {
		progress = &JSONProgress{}
	}
	if sf.json {

		b, err := json.Marshal(&JSONMessage{
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

func (sf *StreamFormatter) Json() bool {
	return sf.json
}

type StdoutFormater struct {
	io.Writer
	*StreamFormatter
}

func (sf *StdoutFormater) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream(string(buf))
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}

type StderrFormater struct {
	io.Writer
	*StreamFormatter
}

func (sf *StderrFormater) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream("\033[91m" + string(buf) + "\033[0m")
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}
