package logrus

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 34
)

func init() {
	baseTimestamp = time.Now()
}

func miniTS() int {
	return int(time.Since(baseTimestamp) / time.Second)
}

type TextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors   bool
	DisableColors bool
}

func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	prefixFieldClashes(entry)

	if (f.ForceColors || IsTerminal()) && !f.DisableColors {
		levelText := strings.ToUpper(entry.Data["level"].(string))[0:4]

		levelColor := blue

		if entry.Data["level"] == "warning" {
			levelColor = yellow
		} else if entry.Data["level"] == "error" ||
			entry.Data["level"] == "fatal" ||
			entry.Data["level"] == "panic" {
			levelColor = red
		}

		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %-44s ", levelColor, levelText, miniTS(), entry.Data["msg"])

		keys := make([]string, 0)
		for k, _ := range entry.Data {
			if k != "level" && k != "time" && k != "msg" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := entry.Data[k]
			fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=%v", levelColor, k, v)
		}
	} else {
		f.AppendKeyValue(b, "time", entry.Data["time"].(string))
		f.AppendKeyValue(b, "level", entry.Data["level"].(string))
		f.AppendKeyValue(b, "msg", entry.Data["msg"].(string))

		for key, value := range entry.Data {
			if key != "time" && key != "level" && key != "msg" {
				f.AppendKeyValue(b, key, value)
			}
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *TextFormatter) AppendKeyValue(b *bytes.Buffer, key, value interface{}) {
	if _, ok := value.(string); ok {
		fmt.Fprintf(b, "%v=%q ", key, value)
	} else {
		fmt.Fprintf(b, "%v=%v ", key, value)
	}
}
