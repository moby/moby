package client

import (
	"net/http"
	"testing"
)

func TestAuthChallengeParse(t *testing.T) {
	header := http.Header{}
	header.Add("WWW-Authenticate", `Bearer realm="https://auth.example.com/token",service="registry.example.com",other=fun,slashed="he\"\l\lo"`)

	challenges := parseAuthHeader(header)
	if len(challenges) != 1 {
		t.Fatalf("Unexpected number of auth challenges: %d, expected 1", len(challenges))
	}

	if expected := "bearer"; challenges[0].Scheme != expected {
		t.Fatalf("Unexpected scheme: %s, expected: %s", challenges[0].Scheme, expected)
	}

	if expected := "https://auth.example.com/token"; challenges[0].Parameters["realm"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenges[0].Parameters["realm"], expected)
	}

	if expected := "registry.example.com"; challenges[0].Parameters["service"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenges[0].Parameters["service"], expected)
	}

	if expected := "fun"; challenges[0].Parameters["other"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenges[0].Parameters["other"], expected)
	}

	if expected := "he\"llo"; challenges[0].Parameters["slashed"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenges[0].Parameters["slashed"], expected)
	}

}
