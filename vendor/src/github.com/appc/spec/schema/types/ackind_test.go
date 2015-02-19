package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestACKindMarshalBad(t *testing.T) {
	tests := map[string]error{
		"Foo": ACKindError("bad ACKind: Foo"),
		"ApplicationManifest": ACKindError("bad ACKind: ApplicationManifest"),
		"": ErrNoACKind,
	}
	for in, werr := range tests {
		a := ACKind(in)
		b, gerr := json.Marshal(a)
		if b != nil {
			t.Errorf("ACKind(%q): want b=nil, got %v", in, b)
		}
		if jerr, ok := gerr.(*json.MarshalerError); !ok {
			t.Errorf("expected JSONMarshalerError")
		} else {
			if e := jerr.Err; e != werr {
				t.Errorf("err=%#v, want %#v", e, werr)
			}
		}
	}
}

func TestACKindMarshalGood(t *testing.T) {
	for i, in := range []string{
		"ImageManifest",
		"ContainerRuntimeManifest",
	} {
		a := ACKind(in)
		b, err := json.Marshal(a)
		if !reflect.DeepEqual(b, []byte(`"`+in+`"`)) {
			t.Errorf("#%d: marshalled=%v, want %v", i, b, []byte(in))
		}
		if err != nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
		}
	}
}
