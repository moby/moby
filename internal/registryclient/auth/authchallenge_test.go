package auth

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
	challenge := challenges[0]

	if expected := "bearer"; challenge.Scheme != expected {
		t.Fatalf("Unexpected scheme: %s, expected: %s", challenge.Scheme, expected)
	}

	if expected := "https://auth.example.com/token"; challenge.Parameters["realm"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenge.Parameters["realm"], expected)
	}

	if expected := "registry.example.com"; challenge.Parameters["service"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenge.Parameters["service"], expected)
	}

	if expected := "fun"; challenge.Parameters["other"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenge.Parameters["other"], expected)
	}

	if expected := "he\"llo"; challenge.Parameters["slashed"] != expected {
		t.Fatalf("Unexpected param: %s, expected: %s", challenge.Parameters["slashed"], expected)
	}

}
