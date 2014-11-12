package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 34
)

var (
	baseTimestamp time.Time
	isTerminal    bool
)

func init() {
	baseTimestamp = time.Now()
	isTerminal = log.IsTerminal()
}

func miniTS() int {
	return int(time.Since(baseTimestamp) / time.Second)
}

func prefixFieldClashes(entry *log.Entry) {
	_, ok := entry.Data["time"]
	if ok {
		entry.Data["fields.time"] = entry.Data["time"]
	}

	_, ok = entry.Data["msg"]
	if ok {
		entry.Data["fields.msg"] = entry.Data["msg"]
	}

	_, ok = entry.Data["level"]
	if ok {
		entry.Data["fields.level"] = entry.Data["level"]
	}
}

type TextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors   bool
	DisableColors bool
}

func (f *TextFormatter) Format(entry *log.Entry) ([]byte, error) {
	var keys []string
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b := &bytes.Buffer{}

	prefixFieldClashes(entry)

	isColored := (f.ForceColors || isTerminal) && !f.DisableColors

	if isColored {
		printColored(b, entry, keys)
	} else {
		f.appendKeyValue(b, "time", entry.Time.Format(time.RFC3339))
		f.appendKeyValue(b, "level", entry.Level.String())
		f.appendKeyValue(b, "msg", entry.Message)
		for _, key := range keys {
			f.appendKeyValue(b, key, entry.Data[key])
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func printColored(b *bytes.Buffer, entry *log.Entry, keys []string) {
	var levelColor int
	switch entry.Level {
	case log.WarnLevel:
		levelColor = yellow
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	levelText := strings.ToUpper(entry.Level.String())[0:4]

	fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %s", levelColor, levelText, miniTS(), entry.Message)
	if len(keys) > 0 {
		fmt.Fprintf(b, " [")
		for _, k := range keys {
			v := entry.Data[k]
			fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=%v", levelColor, k, v)
		}
		fmt.Fprintf(b, " ]")
	}
}

func (f *TextFormatter) appendKeyValue(b *bytes.Buffer, key, value interface{}) {
	switch value.(type) {
	case string, error:
		fmt.Fprintf(b, "%v=%q ", key, value)
	default:
		fmt.Fprintf(b, "%v=%v ", key, value)
	}
}

func initLogging(debug bool, format string) error {
	switch format {
	case "json":
		log.SetFormatter(new(log.JSONFormatter))
	case "std":
		log.SetFormatter(new(TextFormatter))
	default:
		return fmt.Errorf("invalid log format '%s'", format)
	}

	log.SetOutput(os.Stderr)
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	return nil
}
