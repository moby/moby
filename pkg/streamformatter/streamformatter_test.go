package streamformatter // import "github.com/docker/docker/pkg/streamformatter"

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestRawProgressFormatterFormatStatus(t *testing.T) {
	sf := rawProgressFormatter{}
	res := sf.formatStatus("ID", "%s%d", "a", 1)
	assert.Check(t, is.Equal("a1\r\n", string(res)))
}

func TestRawProgressFormatterFormatProgress(t *testing.T) {
	sf := rawProgressFormatter{}
	jsonProgress := &jsonmessage.JSONProgress{
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
	res := FormatStatus("ID", "%s%d", "a", 1)
	expected := `{"status":"a1","id":"ID"}` + streamNewline
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatError(t *testing.T) {
	res := FormatError(errors.New("Error for formatter"))
	expected := `{"errorDetail":{"message":"Error for formatter"},"error":"Error for formatter"}` + "\r\n"
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatJSONError(t *testing.T) {
	err := &jsonmessage.JSONError{Code: 50, Message: "Json error"}
	res := FormatError(err)
	expected := `{"errorDetail":{"code":50,"message":"Json error"},"error":"Json error"}` + streamNewline
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestJsonProgressFormatterFormatProgress(t *testing.T) {
	sf := &jsonProgressFormatter{}
	jsonProgress := &jsonmessage.JSONProgress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	aux := "aux message"
	res := sf.formatProgress("id", "action", jsonProgress, aux)
	msg := &jsonmessage.JSONMessage{}

	assert.NilError(t, json.Unmarshal(res, msg))

	rawAux := json.RawMessage(`"` + aux + `"`)
	expected := &jsonmessage.JSONMessage{
		ID:       "id",
		Status:   "action",
		Aux:      &rawAux,
		Progress: jsonProgress,
	}
	assert.DeepEqual(t, msg, expected, cmpJSONMessageOpt())
}

func cmpJSONMessageOpt() cmp.Option {
	progressMessagePath := func(path cmp.Path) bool {
		return path.String() == "ProgressMessage"
	}
	return cmp.Options{
		cmpopts.IgnoreUnexported(jsonmessage.JSONProgress{}),
		// Ignore deprecated property that is a derivative of Progress
		cmp.FilterPath(progressMessagePath, cmp.Ignore()),
	}
}

func TestJsonProgressFormatterFormatStatus(t *testing.T) {
	sf := jsonProgressFormatter{}
	res := sf.formatStatus("ID", "%s%d", "a", 1)
	assert.Check(t, is.Equal(`{"status":"a1","id":"ID"}`+streamNewline, string(res)))
}

func TestNewJSONProgressOutput(t *testing.T) {
	b := bytes.Buffer{}
	b.Write(FormatStatus("id", "Downloading"))
	_ = NewJSONProgressOutput(&b, false)
	assert.Check(t, is.Equal(`{"status":"Downloading","id":"id"}`+streamNewline, b.String()))
}

func TestAuxFormatterEmit(t *testing.T) {
	b := bytes.Buffer{}
	aux := &AuxFormatter{Writer: &b}
	sampleAux := &struct {
		Data string
	}{"Additional data"}
	err := aux.Emit(sampleAux)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(`{"aux":{"Data":"Additional data"}}`+streamNewline, b.String()))
}
