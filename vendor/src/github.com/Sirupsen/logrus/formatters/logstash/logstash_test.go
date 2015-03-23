package logstash

import (
	"bytes"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLogstashFormatter(t *testing.T) {
	assert := assert.New(t)

	lf := LogstashFormatter{Type: "abc"}

	fields := logrus.Fields{
		"message": "def",
		"level":   "ijk",
		"type":    "lmn",
		"one":     1,
		"pi":      3.14,
		"bool":    true,
	}

	entry := logrus.WithFields(fields)
	entry.Message = "msg"
	entry.Level = logrus.InfoLevel

	b, _ := lf.Format(entry)

	var data map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.Decode(&data)

	// base fields
	assert.Equal(json.Number("1"), data["@version"])
	assert.NotEmpty(data["@timestamp"])
	assert.Equal("abc", data["type"])
	assert.Equal("msg", data["message"])
	assert.Equal("info", data["level"])

	// substituted fields
	assert.Equal("def", data["fields.message"])
	assert.Equal("ijk", data["fields.level"])
	assert.Equal("lmn", data["fields.type"])

	// formats
	assert.Equal(json.Number("1"), data["one"])
	assert.Equal(json.Number("3.14"), data["pi"])
	assert.Equal(true, data["bool"])
}
