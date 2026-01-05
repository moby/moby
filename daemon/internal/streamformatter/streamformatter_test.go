package streamformatter

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/moby/moby/api/types/jsonstream"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestFormatStatus(t *testing.T) {
	res := FormatStatus("ID", "%s%d", "a", 1)
	expected := `{"status":"a1","id":"ID"}` + streamNewline
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatError(t *testing.T) {
	res := FormatError(errors.New("Error for formatter"))
	expected := `{"error":"Error for formatter","errorDetail":{"message":"Error for formatter"}}` + "\r\n"
	assert.Check(t, is.Equal(expected, string(res)))
}

func TestFormatJSONError(t *testing.T) {
	err := &jsonstream.Error{Code: 50, Message: "Json error"}
	res := FormatError(err)
	expected := `{"error":"Json error","errorDetail":{"code":50,"message":"Json error"}}` + streamNewline
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
	assert.DeepEqual(t, msg, expected, cmpJSONMessageOpt())
}

func cmpJSONMessageOpt() cmp.Option {
	progressMessagePath := func(path cmp.Path) bool {
		return path.String() == "ProgressMessage"
	}
	return cmp.Options{
		// Ignore deprecated property that is a derivative of Progress
		cmp.FilterPath(progressMessagePath, cmp.Ignore()),
	}
}

func TestJsonProgressFormatterFormatStatus(t *testing.T) {
	sf := jsonProgressFormatter{}
	res := sf.formatStatus("ID", "%s%d", "a", 1)
	assert.Check(t, is.Equal(`{"status":"a1","id":"ID"}`+streamNewline, string(res)))
}

func TestJsonProgressFormatterFormatStatusWithPushResult(t *testing.T) {
	sf := jsonProgressFormatter{}
	pushResult := &jsonstream.PushResult{
		Tag:    "latest",
		Digest: "sha256:deadbeef",
		Size:   1234,
	}
	res := sf.formatStatusWithAux("ID", "done", pushResult)
	msg := &jsonstream.Message{}

	assert.NilError(t, json.Unmarshal(res, msg))

	expected := &jsonstream.Message{
		ID:     "ID",
		Status: "done",
		Push:   pushResult,
	}
	assert.DeepEqual(t, msg, expected, cmpJSONMessageOpt())
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
	err := aux.Emit("", sampleAux)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(`{"aux":{"Data":"Additional data"}}`+streamNewline, b.String()))
}
