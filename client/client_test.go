package docker

import (
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type dumb struct {
	x int
	y float64
}

func TestNewAPIClient(t *testing.T) {
	endpoint := "http://localhost:4243"
	client, err := NewClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint %s. Got %s.", endpoint, client.endpoint)
	}
	if client.client != http.DefaultClient {
		t.Errorf("Expected http.Client %#v. Got %#v.", http.DefaultClient, client.client)
	}
	_, err = NewClient("")
	if err == nil {
		t.Fatal("Unexpected <nil> error")
	}
}

func TestAPIClientGetURL(t *testing.T) {
	var tests = []struct {
		endpoint string
		path     string
		expected string
	}{
		{"http://localhost:4243/", "/", "http://localhost:4243/"},
		{"http://localhost:4243", "/", "http://localhost:4243/"},
		{"http://localhost:4243", "/containers/ps", "http://localhost:4243/containers/ps"},
		{"http://localhost:4243/////", "/", "http://localhost:4243/"},
	}
	var client Client
	for _, tt := range tests {
		client.endpoint = tt.endpoint
		got := client.getURL(tt.path)
		if got != tt.expected {
			t.Errorf("getURL(%q): Got %s. Want %s.", tt.path, got, tt.expected)
		}
	}
}

func TestAPIClientError(t *testing.T) {
	resp := http.Response{StatusCode: 400, Body: ioutil.NopCloser(strings.NewReader("bad parameter"))}
	err := newApiClientError(&resp)
	expected := apiClientError{status: 400, message: "bad parameter"}
	if !reflect.DeepEqual(expected, *err) {
		t.Errorf("Wrong error type. Want %#v. Got %#v.", expected, *err)
	}
	message := "API error (400): bad parameter"
	if err.Error() != message {
		t.Errorf("Wrong error message. Want %q. Got %q.", message, err.Error())
	}
}

func TestAPIClientQueryString(t *testing.T) {
	var tests = []struct {
		input interface{}
		want  string
	}{
		{&ListContainersOptions{All: true}, "all=1"},
		{ListContainersOptions{All: true}, "all=1"},
		{ListContainersOptions{Before: "something"}, "before=something"},
		{ListContainersOptions{Before: "something", Since: "other"}, "before=something&since=other"},
		{dumb{x: 10, y: 10.35000}, "x=10&y=10.35"},
		{nil, ""},
		{10, ""},
		{"not_a_struct", ""},
	}
	for _, tt := range tests {
		got := queryString(tt.input)
		if got != tt.want {
			t.Errorf("queryString(%v). Want %q. Got %q.", tt.input, tt.want, got)
		}
	}
}

type FakeRoundTripper struct {
	message  string
	status   int
	requests []*http.Request
}

func (rt *FakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.NewReader(rt.message)
	rt.requests = append(rt.requests, r)
	return &http.Response{
		StatusCode: rt.status,
		Body:       ioutil.NopCloser(body),
	}, nil
}

func (rt *FakeRoundTripper) Reset() {
	rt.requests = nil
}
