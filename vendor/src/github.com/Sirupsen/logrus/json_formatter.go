package logrus

import (
	"encoding/json"
	"fmt"
	"time"
)

type JSONFormatter struct{}

func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	prefixFieldClashes(entry)
	entry.Data["time"] = entry.Time.Format(time.RFC3339)
	entry.Data["msg"] = entry.Message
	entry.Data["level"] = entry.Level.String()

	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}
