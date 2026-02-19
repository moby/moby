// Package streamformatter provides helper functions to format a stream.
package streamformatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client/pkg/progress"
)

const streamNewline = "\r\n"

func appendNewline(source []byte) []byte {
	return append(source, '\r', '\n')
}

type jsonProgressFormatter struct{}

// formatStatus formats the id and status.
func (sf *jsonProgressFormatter) formatStatus(id, status string) []byte {
	b, err := json.Marshal(&jsonstream.Message{
		ID:     id,
		Status: status,
	})
	if err != nil {
		// should never happen with the given struct.
		return nil
	}
	return appendNewline(b)
}

// formatProgress formats the progress information for a specified action.
func (sf *jsonProgressFormatter) formatProgress(id, action string, progress *jsonstream.Progress, aux any) []byte {
	var auxJSON *json.RawMessage
	if aux != nil {
		b, err := json.Marshal(aux)
		if err != nil {
			return nil
		}
		auxJSON = (*json.RawMessage)(&b)
	}
	if progress == nil {
		progress = &jsonstream.Progress{}
	}
	b, err := json.Marshal(&jsonstream.Message{
		Status:   action,
		Progress: progress,
		ID:       id,
		Aux:      auxJSON,
	})
	if err != nil {
		return nil
	}
	return appendNewline(b)
}

type rawProgressFormatter struct{}

func (sf *rawProgressFormatter) formatStatus(id, status string) []byte {
	return []byte(status + streamNewline)
}

func rawProgressString(p *jsonstream.Progress) string {
	if p == nil || (p.Current <= 0 && p.Total <= 0) {
		return ""
	}
	if p.Total <= 0 {
		switch p.Units {
		case "":
			return fmt.Sprintf("%8v", units.HumanSize(float64(p.Current)))
		default:
			return fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	percentage := int(float64(p.Current)/float64(p.Total)*100) / 2
	if percentage > 50 {
		percentage = 50
	}

	numSpaces := 0
	if 50-percentage > 0 {
		numSpaces = 50 - percentage
	}
	pbBox := fmt.Sprintf("[%s>%s] ", strings.Repeat("=", percentage), strings.Repeat(" ", numSpaces))

	var numbersBox string
	switch {
	case p.HideCounts:
	case p.Units == "": // no units, use bytes
		current := units.HumanSize(float64(p.Current))
		total := units.HumanSize(float64(p.Total))

		numbersBox = fmt.Sprintf("%8v/%v", current, total)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%8v", current)
		}
	default:
		numbersBox = fmt.Sprintf("%d/%d %s", p.Current, p.Total, p.Units)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	var timeLeftBox string
	if p.Current > 0 && p.Start > 0 && percentage < 50 {
		fromStart := time.Since(time.Unix(p.Start, 0))
		perEntry := fromStart / time.Duration(p.Current)
		left := time.Duration(p.Total-p.Current) * perEntry
		timeLeftBox = " " + left.Round(time.Second).String()
	}
	return pbBox + numbersBox + timeLeftBox
}

func (sf *rawProgressFormatter) formatProgress(id, action string, progress *jsonstream.Progress, aux any) []byte {
	endl := "\r"
	out := rawProgressString(progress)
	if out == "" {
		endl += "\n"
	}
	return []byte(action + " " + out + endl)
}

// NewProgressOutput returns a progress.Output object that can be passed to
// progress.NewProgressReader.
func NewProgressOutput(out io.Writer) progress.Output {
	return &progressOutput{sf: &rawProgressFormatter{}, out: out, newLines: true}
}

// NewJSONProgressOutput returns a progress.Output that formats output
// using JSON objects
func NewJSONProgressOutput(out io.Writer, newLines bool) progress.Output {
	return &progressOutput{sf: &jsonProgressFormatter{}, out: out, newLines: newLines}
}

type formatProgress interface {
	formatStatus(id, status string) []byte
	formatProgress(id, action string, progress *jsonstream.Progress, aux any) []byte
}

type progressOutput struct {
	sf  formatProgress
	out io.Writer

	// TODO(thaJeztah): investigate if this can be removed or replaced.
	//
	// It was a workaround for responses adding an extra (final) (aux) message
	// progress; see https://github.com/moby/moby/pull/1425. When updating, also
	// check for the similar implementation in daemon/internal/streamformatter.
	newLines bool
	mu       sync.Mutex
}

// WriteProgress formats progress information from a ProgressReader.
func (out *progressOutput) WriteProgress(prog progress.Progress) error {
	var formatted []byte
	if prog.Message != "" {
		formatted = out.sf.formatStatus(prog.ID, prog.Message)
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
