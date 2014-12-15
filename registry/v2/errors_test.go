package v2

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestErrorCodes ensures that error code format, mappings and
// marshaling/unmarshaling. round trips are stable.
func TestErrorCodes(t *testing.T) {
	for _, desc := range ErrorDescriptors {
		if desc.Code.String() != desc.Value {
			t.Fatalf("error code string incorrect: %q != %q", desc.Code.String(), desc.Value)
		}

		if desc.Code.Message() != desc.Message {
			t.Fatalf("incorrect message for error code %v: %q != %q", desc.Code, desc.Code.Message(), desc.Message)
		}

		// Serialize the error code using the json library to ensure that we
		// get a string and it works round trip.
		p, err := json.Marshal(desc.Code)

		if err != nil {
			t.Fatalf("error marshaling error code %v: %v", desc.Code, err)
		}

		if len(p) <= 0 {
			t.Fatalf("expected content in marshaled before for error code %v", desc.Code)
		}

		// First, unmarshal to interface and ensure we have a string.
		var ecUnspecified interface{}
		if err := json.Unmarshal(p, &ecUnspecified); err != nil {
			t.Fatalf("error unmarshaling error code %v: %v", desc.Code, err)
		}

		if _, ok := ecUnspecified.(string); !ok {
			t.Fatalf("expected a string for error code %v on unmarshal got a %T", desc.Code, ecUnspecified)
		}

		// Now, unmarshal with the error code type and ensure they are equal
		var ecUnmarshaled ErrorCode
		if err := json.Unmarshal(p, &ecUnmarshaled); err != nil {
			t.Fatalf("error unmarshaling error code %v: %v", desc.Code, err)
		}

		if ecUnmarshaled != desc.Code {
			t.Fatalf("unexpected error code during error code marshal/unmarshal: %v != %v", ecUnmarshaled, desc.Code)
		}
	}
}

// TestErrorsManagement does a quick check of the Errors type to ensure that
// members are properly pushed and marshaled.
func TestErrorsManagement(t *testing.T) {
	var errs Errors

	errs.Push(ErrorCodeDigestInvalid)
	errs.Push(ErrorCodeBlobUnknown,
		map[string]string{"digest": "sometestblobsumdoesntmatter"})

	p, err := json.Marshal(errs)

	if err != nil {
		t.Fatalf("error marashaling errors: %v", err)
	}

	expectedJSON := "{\"errors\":[{\"code\":\"DIGEST_INVALID\",\"message\":\"provided digest did not match uploaded content\"},{\"code\":\"BLOB_UNKNOWN\",\"message\":\"blob unknown to registry\",\"detail\":{\"digest\":\"sometestblobsumdoesntmatter\"}}]}"

	if string(p) != expectedJSON {
		t.Fatalf("unexpected json: %q != %q", string(p), expectedJSON)
	}

	errs.Clear()
	errs.Push(ErrorCodeUnknown)
	expectedJSON = "{\"errors\":[{\"code\":\"UNKNOWN\",\"message\":\"unknown error\"}]}"
	p, err = json.Marshal(errs)

	if err != nil {
		t.Fatalf("error marashaling errors: %v", err)
	}

	if string(p) != expectedJSON {
		t.Fatalf("unexpected json: %q != %q", string(p), expectedJSON)
	}
}

// TestMarshalUnmarshal ensures that api errors can round trip through json
// without losing information.
func TestMarshalUnmarshal(t *testing.T) {

	var errors Errors

	for _, testcase := range []struct {
		description string
		err         Error
	}{
		{
			description: "unknown error",
			err: Error{

				Code:    ErrorCodeUnknown,
				Message: ErrorCodeUnknown.Descriptor().Message,
			},
		},
		{
			description: "unknown manifest",
			err: Error{
				Code:    ErrorCodeManifestUnknown,
				Message: ErrorCodeManifestUnknown.Descriptor().Message,
			},
		},
		{
			description: "unknown manifest",
			err: Error{
				Code:    ErrorCodeBlobUnknown,
				Message: ErrorCodeBlobUnknown.Descriptor().Message,
				Detail:  map[string]interface{}{"digest": "asdfqwerqwerqwerqwer"},
			},
		},
	} {
		fatalf := func(format string, args ...interface{}) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

		unexpectedErr := func(err error) {
			fatalf("unexpected error: %v", err)
		}

		p, err := json.Marshal(testcase.err)
		if err != nil {
			unexpectedErr(err)
		}

		var unmarshaled Error
		if err := json.Unmarshal(p, &unmarshaled); err != nil {
			unexpectedErr(err)
		}

		if !reflect.DeepEqual(unmarshaled, testcase.err) {
			fatalf("errors not equal after round trip: %#v != %#v", unmarshaled, testcase.err)
		}

		// Roll everything up into an error response envelope.
		errors.PushErr(testcase.err)
	}

	p, err := json.Marshal(errors)
	if err != nil {
		t.Fatalf("unexpected error marshaling error envelope: %v", err)
	}

	var unmarshaled Errors
	if err := json.Unmarshal(p, &unmarshaled); err != nil {
		t.Fatalf("unexpected error unmarshaling error envelope: %v", err)
	}

	if !reflect.DeepEqual(unmarshaled, errors) {
		t.Fatalf("errors not equal after round trip: %#v != %#v", unmarshaled, errors)
	}
}
