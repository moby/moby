package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"sync"
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

func TestAuthChallengeNormalization(t *testing.T) {
	testAuthChallengeNormalization(t, "reg.EXAMPLE.com")
	testAuthChallengeNormalization(t, "bɿɒʜɔiɿ-ɿɘƚƨim-ƚol-ɒ-ƨʞnɒʜƚ.com")
	testAuthChallengeConcurrent(t, "reg.EXAMPLE.com")
}

func testAuthChallengeNormalization(t *testing.T, host string) {

	scm := NewSimpleChallengeManager()

	url, err := url.Parse(fmt.Sprintf("http://%s/v2/", host))
	if err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		Request: &http.Request{
			URL: url,
		},
		Header:     make(http.Header),
		StatusCode: http.StatusUnauthorized,
	}
	resp.Header.Add("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"https://%s/token\",service=\"registry.example.com\"", host))

	err = scm.AddResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	lowered := *url
	lowered.Host = strings.ToLower(lowered.Host)
	c, err := scm.GetChallenges(lowered)
	if err != nil {
		t.Fatal(err)
	}

	if len(c) == 0 {
		t.Fatal("Expected challenge for lower-cased-host URL")
	}
}


func testAuthChallengeConcurrent(t *testing.T, host string) {

	scm := NewSimpleChallengeManager()

	url, err := url.Parse(fmt.Sprintf("http://%s/v2/", host))
	if err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		Request: &http.Request{
			URL: url,
		},
		Header:     make(http.Header),
		StatusCode: http.StatusUnauthorized,
	}
	resp.Header.Add("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"https://%s/token\",service=\"registry.example.com\"", host))
	var s sync.WaitGroup
	s.Add(2)
	go func() {
		i := 200
		for {
			//time.Sleep(500 * time.Millisecond)
			err = scm.AddResponse(resp)
			if err != nil {
				t.Fatal(err)
			}
			i = i -1
			if i < 0{
				break
			}
		}
		s.Done()
	}()
	go func() {
		lowered := *url
		lowered.Host = strings.ToLower(lowered.Host)
		k:= 200
		for {
			_, err := scm.GetChallenges(lowered)
			if err != nil {
				t.Fatal(err)
			}
			k = k -1
			if k < 0{
				break
			}
		}

		s.Done()
	}()
	s.Wait()
}
