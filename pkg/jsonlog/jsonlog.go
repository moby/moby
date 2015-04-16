package jsonlog

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	log "github.com/Sirupsen/logrus"
)

type JSONLog struct {
	Log     string    `json:"log,omitempty"`
	Stream  string    `json:"stream,omitempty"`
	Created time.Time `json:"time"`
}

func (jl *JSONLog) Format(format string) (string, error) {
	if format == "" {
		return jl.Log, nil
	}
	if format == "json" {
		m, err := json.Marshal(jl)
		return string(m), err
	}
	return fmt.Sprintf("%s %s", jl.Created.Format(format), jl.Log), nil
}

func (jl *JSONLog) Reset() {
	jl.Log = ""
	jl.Stream = ""
	jl.Created = time.Time{}
}

func WriteLog(src io.Reader, dst io.Writer, format string) error {
	dec := json.NewDecoder(src)
	l := &JSONLog{}
	for {
		if err := dec.Decode(l); err == io.EOF {
			return nil
		} else if err != nil {
			log.Printf("Error streaming logs: %s", err)
			return err
		}
		line, err := l.Format(format)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(dst, line); err != nil {
			return err
		}
		l.Reset()
	}
}
