// Package streamformatter provides helper functions to format a stream.
package streamformatter

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"github.com/moby/moby/v2/daemon/internal/progress"
)

const streamNewline = "\r\n"

type jsonProgressFormatter struct{}

func appendNewline(source []byte) []byte {
	return append(source, []byte(streamNewline)...)
}

func pushResultFromAux(aux any) *jsonstream.PushResult {
	switch v := aux.(type) {
	case jsonstream.PushResult:
		return &v
	case *jsonstream.PushResult:
		return v
	default:
		return nil
	}
}

// FormatStatus formats the specified objects according to the specified format (and id).
func FormatStatus(id, format string, a ...any) []byte {
	str := fmt.Sprintf(format, a...)
	b, err := json.Marshal(&jsonstream.Message{ID: id, Status: str})
	if err != nil {
		return FormatError(err)
	}
	return appendNewline(b)
}

// FormatError formats the error as a JSON object
func FormatError(err error) []byte {
	jsonError, ok := err.(*jsonstream.Error)
	if !ok {
		jsonError = &jsonstream.Error{Message: err.Error()}
	}
	if b, err := json.Marshal(compat.Wrap(&jsonstream.Message{Error: jsonError}, compat.WithExtraFields(map[string]any{"error": jsonError.Error()}))); err == nil {
		return appendNewline(b)
	}
	return []byte(`{"error":"format error"}` + streamNewline)
}

func (sf *jsonProgressFormatter) formatStatus(id, format string, a ...any) []byte {
	return FormatStatus(id, format, a...)
}

func (sf *jsonProgressFormatter) formatStatusWithAux(id, message string, aux any) []byte {
	pushResult := pushResultFromAux(aux)
	if pushResult == nil {
		return nil
	}
	b, err := json.Marshal(&jsonstream.Message{ID: id, Status: message, Push: pushResult})
	if err != nil {
		return nil
	}
	return appendNewline(b)
}

// formatProgress formats the progress information for a specified action.
func (sf *jsonProgressFormatter) formatProgress(id, action string, progress *jsonstream.Progress, aux any) []byte {
	if progress == nil {
		progress = &jsonstream.Progress{}
	}
	pushResult := pushResultFromAux(aux)
	var auxJSON *json.RawMessage
	if aux != nil && pushResult == nil {
		auxJSONBytes, err := json.Marshal(aux)
		if err != nil {
			return nil
		}
		auxJSON = new(json.RawMessage)
		*auxJSON = auxJSONBytes
	}
	b, err := json.Marshal(&jsonstream.Message{
		Status:   action,
		Progress: progress,
		ID:       id,
		Push:     pushResult,
		Aux:      auxJSON,
	})
	if err != nil {
		return nil
	}
	return appendNewline(b)
}

// NewJSONProgressOutput returns a progress.Output that formats output
// using JSON objects
func NewJSONProgressOutput(out io.Writer, newLines bool) progress.Output {
	return &progressOutput{sf: &jsonProgressFormatter{}, out: out, newLines: newLines}
}

type formatProgress interface {
	formatStatus(id, format string, a ...any) []byte
	formatProgress(id, action string, progress *jsonstream.Progress, aux any) []byte
}

type formatStatusWithAux interface {
	formatStatusWithAux(id, message string, aux any) []byte
}

type progressOutput struct {
	sf       formatProgress
	out      io.Writer
	newLines bool
	mu       sync.Mutex
}

// WriteProgress formats progress information from a ProgressReader.
func (out *progressOutput) WriteProgress(prog progress.Progress) error {
	var formatted []byte
	if prog.Message != "" {
		if prog.Aux != nil {
			if formatter, ok := out.sf.(formatStatusWithAux); ok {
				formatted = formatter.formatStatusWithAux(prog.ID, prog.Message, prog.Aux)
			}
		}
		if formatted == nil {
			formatted = out.sf.formatStatus(prog.ID, prog.Message)
		}
	} else {
		jsonProgress := jsonstream.Progress{
			Current:    prog.Current,
			Total:      prog.Total,
			HideCounts: prog.HideCounts,
			Units:      prog.Units,
		}
		formatted = out.sf.formatProgress(prog.ID, prog.Action, &jsonProgress, prog.Aux)
	}

	out.mu.Lock()
	defer out.mu.Unlock()
	_, err := out.out.Write(formatted)
	if err != nil {
		return err
	}

	if out.newLines && prog.LastUpdate {
		_, err = out.out.Write(out.sf.formatStatus("", ""))
		return err
	}

	return nil
}

// AuxFormatter is a streamFormatter that writes aux progress messages
type AuxFormatter struct {
	io.Writer
}

// Emit emits the given interface as an aux progress message
func (sf *AuxFormatter) Emit(id string, aux any) error {
	auxJSONBytes, err := json.Marshal(aux)
	if err != nil {
		return err
	}
	auxJSON := new(json.RawMessage)
	*auxJSON = auxJSONBytes
	msgJSON, err := json.Marshal(&jsonstream.Message{ID: id, Aux: auxJSON})
	if err != nil {
		return err
	}
	msgJSON = appendNewline(msgJSON)
	n, err := sf.Writer.Write(msgJSON)
	if n != len(msgJSON) {
		return io.ErrShortWrite
	}
	return err
}
