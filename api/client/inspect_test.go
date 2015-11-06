package client

import (
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

func TestDecodeRawInspect(t *testing.T) {
	cases := []struct {
		input  string
		tmpl   string
		fail   bool
		output string
	}{
		{`{"HostConfig": {"Dns": "8.8.8.8"}}`, "{{.HostConfig.Dns}}", false, "8.8.8.8"},
		{`{"HostConfig": {"Dns": null}}`, "{{.HostConfig.Dns}}", false, "<no value>"},
		{`{"HostConfig": {"Dns": "8.8.8.8"}}`, "{{.HostConfig.Foo}}", true, ""},
	}

	for _, cs := range cases {
		r := strings.NewReader(cs.input)
		d := json.NewDecoder(r)

		tmpl, err := template.New("").Parse(cs.tmpl)
		if err != nil {
			t.Fatal(err)
		}

		out, err := decodeRawInspect(tmpl, d)
		if cs.fail {
			if err == nil {
				t.Fatalf("Template parsing expected to fail, got nil error. Template: %s, input: %s", cs.tmpl, cs.input)
			}
		} else {
			if out.String() != cs.output {
				t.Fatalf("Template parsing expected output %s, got %s", cs.output, out.String())
			}
		}

	}
}
