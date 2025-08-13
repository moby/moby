package container

import (
	"testing"
)

func TestPortProto(t *testing.T) {
	p := PortProto("1234/tcp")

	if string(p) != "1234/tcp" {
		t.Fatal("tcp, 1234 did not result in the string 1234/tcp")
	}

	if p.Proto() != "tcp" {
		t.Fatal("protocol was not tcp")
	}

	if p.Port() != "1234" {
		t.Fatal("port string value was not 1234")
	}

	if portNum, err := p.Int(); err != nil {
		t.Fatalf("port int value had an error: %v", err)
	} else if portNum != 1234 {
		t.Fatal("port int value was not 1234")
	}

	p = PortProto("1234")

	if p.Proto() != "tcp" {
		t.Fatal("default protocol was not tcp")
	}

	if p.Port() != "1234" {
		t.Fatal("port string value was not 1234")
	}

	if portNum, err := p.Int(); err != nil {
		t.Fatalf("port int value had an error: %v", err)
	} else if portNum != 1234 {
		t.Fatal("port int value was not 1234")
	}

	p = PortProto("asd1234/tcp")
	if portNum, err := p.Int(); err == nil {
		t.Fatalf("port int value was supposed to have an error, got %d", portNum)
	}

	p = PortProto("1234-1230/tcp")
	if portNum, err := p.Int(); err == nil {
		t.Fatalf("port int value was supposed to have an error, got %d", portNum)
	}

	p = PortProto("65536/tcp")
	if portNum, err := p.Int(); err == nil {
		t.Fatalf("port int value was supposed to have an error, got %d", portNum)
	}
}

func TestPortRangeProto(t *testing.T) {
	pr := PortRangeProto("1234-1240/tcp")

	if string(pr) != "1234-1240/tcp" {
		t.Fatal("tcp, 1234-1240 did not result in the string 1234-1240/tcp")
	}

	if pr.Proto() != "tcp" {
		t.Fatal("protocol was not tcp")
	}

	if pr.PortRange() != "1234-1240" {
		t.Fatal("port range string value was not 1234-1240")
	}

	if start, end, err := pr.Range(); err != nil {
		t.Fatalf("port range int value had an error: %v", err)
	} else if start != 1234 || end != 1240 {
		t.Fatalf("port range int value was not 1234-1240, got %d-%d", start, end)
	}

	pr = PortRangeProto("1234-1240")

	if pr.Proto() != "tcp" {
		t.Fatal("default protocol was not tcp")
	}

	if pr.PortRange() != "1234-1240" {
		t.Fatal("port range string value was not 1234-1240")
	}

	if start, end, err := pr.Range(); err != nil {
		t.Fatalf("port range int value had an error: %v", err)
	} else if start != 1234 || end != 1240 {
		t.Fatalf("port range int value was not 1234-1240, got %d-%d", start, end)
	}

	pr = PortRangeProto("/tcp")
	if start, end, err := pr.Range(); err != nil {
		t.Fatalf("port range int value had an error: %v", err)
	} else if start != 0 || end != 0 {
		t.Fatalf("port range int value was not 0-0, got %d-%d", start, end)
	}

	pr = PortRangeProto("1234/tcp")
	if start, end, err := pr.Range(); err != nil {
		t.Fatalf("port range int value had an error: %v", err)
	} else if start != 1234 || end != 1234 {
		t.Fatalf("port range int value was not 1234-1234, got %d-%d", start, end)
	}

	pr = PortRangeProto("asd1234-1240/tcp")
	if start, end, err := pr.Range(); err == nil {
		t.Fatalf("port range int value was supposed to have an error, got %d-%d", start, end)
	}

	pr = PortRangeProto("1234-asd1240/tcp")
	if start, end, err := pr.Range(); err == nil {
		t.Fatalf("port range int value was supposed to have an error, got %d-%d", start, end)
	}

	pr = PortRangeProto("1240-1234/tcp")
	if start, end, err := pr.Range(); err == nil {
		t.Fatalf("port range int value was supposed to have an error, got %d-%d", start, end)
	}
}

func TestParsePortNumber(t *testing.T) {
	tests := []struct {
		doc    string
		input  string
		exp    int
		expErr string
	}{
		{
			doc:    "empty string",
			input:  "",
			expErr: "value is empty",
		},
		{
			doc:    "whitespace only",
			input:  "   ",
			expErr: "invalid syntax",
		},
		{
			doc:   "single valid port",
			input: "1234",
			exp:   1234,
		},
		{
			doc:   "zero port",
			input: "0",
			exp:   0,
		},
		{
			doc:   "max valid port",
			input: "65535",
			exp:   65535,
		},
		{
			doc:    "negative port",
			input:  "-1",
			expErr: "value out of range (0–65535)",
		},
		{
			doc:    "too large port",
			input:  "65536",
			expErr: "value out of range (0–65535)",
		},
		{
			doc:    "non-numeric",
			input:  "foo",
			expErr: "invalid syntax",
		},
		{
			doc:    "trailing garbage",
			input:  "1234abc",
			expErr: "invalid syntax",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			got, err := parsePortNumber(tc.input)

			if tc.expErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %q", err)
				}
				if got != tc.exp {
					t.Errorf("expected %d, got %d", tc.exp, got)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.expErr)
				}
				if err.Error() != tc.expErr {
					t.Errorf("expected error %q, got %q", tc.expErr, err.Error())
				}
			}
		})
	}
}
