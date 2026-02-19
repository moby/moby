package streamformatter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client/pkg/progress"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRawProgressFormatterFormatStatus(t *testing.T) {
	sf := rawProgressFormatter{}
	res := sf.formatStatus("ID", "%s%d", "a", 1)
	assert.Check(t, is.Equal("a1\r\n", string(res)))
}

func TestRawProgressFormatterFormatProgress(t *testing.T) {
	sf := rawProgressFormatter{}
	jsonProgress := &jsonstream.Progress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	res := sf.formatProgress("id", "action", jsonProgress, nil)
	out := string(res)
	assert.Check(t, strings.HasPrefix(out, "action [===="))
	assert.Check(t, is.Contains(out, "15B/30B"))
	assert.Check(t, strings.HasSuffix(out, "\r"))
}

func TestFormatStatus(t *testing.T) {
	res := formatStatus("ID", "%s%d", "a", 1)
	expected := `{"status":"a1","id":"ID"}` + streamNewline
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatError(t *testing.T) {
	res := formatError(errors.New("Error for formatter"))
	expected := `{"errorDetail":{"message":"Error for formatter"}}` + "\r\n"
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatJSONError(t *testing.T) {
	err := &jsonstream.Error{Code: 50, Message: "Json error"}
	res := formatError(err)
	expected := `{"errorDetail":{"code":50,"message":"Json error"}}` + streamNewline
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestJsonProgressFormatterFormatProgress(t *testing.T) {
	sf := &jsonProgressFormatter{}
	jsonProgress := &jsonstream.Progress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	aux := "aux message"
	res := sf.formatProgress("id", "action", jsonProgress, aux)
	msg := &jsonstream.Message{}

	assert.NilError(t, json.Unmarshal(res, msg))

	rawAux := json.RawMessage(`"` + aux + `"`)
	expected := &jsonstream.Message{
		ID:       "id",
		Status:   "action",
		Aux:      &rawAux,
		Progress: jsonProgress,
	}
	assert.DeepEqual(t, msg, expected)
}

func TestJsonProgressFormatterFormatStatus(t *testing.T) {
	sf := jsonProgressFormatter{}
	res := sf.formatStatus("ID", "%s%d", "a", 1)
	assert.Check(t, is.Equal(`{"status":"a1","id":"ID"}`+streamNewline, string(res)))
}

func TestJSONProgressOutputWriteProgress(t *testing.T) {
	tests := []struct {
		doc        string
		newlines   bool
		lastUpdate bool
		expected   string
	}{
		{
			doc:      "no newlines",
			expected: `{"status":"Downloading","id":"id"}` + streamNewline,
		},
		{
			doc:        "no newlines last update",
			lastUpdate: true,
			expected:   `{"status":"Downloading","id":"id"}` + streamNewline,
		},
		{
			doc:        "newlines",
			newlines:   true,
			lastUpdate: false,
			expected:   `{"status":"Downloading","id":"id"}` + streamNewline,
		},
		{
			// Should print an extra (empty) message to add newlines after last message
			// (LastUpdate=true); see https://github.com/moby/moby/pull/1425
			doc:        "newlines last update",
			newlines:   true,
			lastUpdate: true,
			expected:   `{"status":"Downloading","id":"id"}` + streamNewline + `{}` + streamNewline,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			var b bytes.Buffer
			po := NewJSONProgressOutput(&b, tc.newlines)

			err := po.WriteProgress(progress.Progress{
				ID:         "id",
				Message:    "Downloading",
				LastUpdate: tc.lastUpdate,
			})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(b.String(), tc.expected))
		})
	}
}
