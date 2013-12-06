package utils

import (
	"encoding/json"
	"fmt"
)

type StreamFormatter struct {
	json bool
	used bool
}

func NewStreamFormatter(json bool) *StreamFormatter {
	return &StreamFormatter{json, false}
}

func (sf *StreamFormatter) FormatStream(str string) []byte {
	sf.used = true
	if sf.json {
		b, err := json.Marshal(&JSONMessage{Stream: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return b
	}
	return []byte(str + "\r")
}

func (sf *StreamFormatter) FormatStatus(id, format string, a ...interface{}) []byte {
	sf.used = true
	str := fmt.Sprintf(format, a...)
	if sf.json {
		b, err := json.Marshal(&JSONMessage{ID: id, Status: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return b
	}
	return []byte(str + "\r\n")
}

func (sf *StreamFormatter) FormatError(err error) []byte {
	sf.used = true
	if sf.json {
		jsonError, ok := err.(*JSONError)
		if !ok {
			jsonError = &JSONError{Message: err.Error()}
		}
		if b, err := json.Marshal(&JSONMessage{Error: jsonError, ErrorMessage: err.Error()}); err == nil {
			return b
		}
		return []byte("{\"error\":\"format error\"}")
	}
	return []byte("Error: " + err.Error() + "\r\n")
}

func (sf *StreamFormatter) FormatProgress(id, action string, progress *JSONProgress) []byte {
	if progress == nil {
		progress = &JSONProgress{}
	}
	sf.used = true
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

func (sf *StreamFormatter) Used() bool {
	return sf.used
}
