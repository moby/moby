package streamformatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client/pkg/progress"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRawProgressFormatterFormatStatus(t *testing.T) {
	res := new(rawProgressFormatter).formatStatus("ID", "status")
	expected := "status" + streamNewline
	assert.Check(t, is.Equal(string(res), expected))
}

func TestRawProgressFormatterFormatProgress(t *testing.T) {
	jsonProgress := &jsonstream.Progress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	res := new(rawProgressFormatter).formatProgress("id", "action", jsonProgress, nil)
	out := string(res)
	assert.Check(t, strings.HasPrefix(out, "action [===="))
	assert.Check(t, is.Contains(out, "15B/30B"))
	assert.Check(t, strings.HasSuffix(out, "\r"))
}

func TestJSONProgressFormatterFormatProgress(t *testing.T) {
	jsonProgress := &jsonstream.Progress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	aux := "aux message"
	res := new(jsonProgressFormatter).formatProgress("id", "action", jsonProgress, aux)
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

func TestJSONProgressFormatterFormatStatus(t *testing.T) {
	res := new(jsonProgressFormatter).formatStatus("ID", "status")
	expected := `{"status":"status","id":"ID"}` + streamNewline
	assert.Check(t, is.Equal(string(res), expected))
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
