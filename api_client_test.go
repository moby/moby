package docker

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	endpoint := "http://localhost:4243"
	client, err := NewAPIClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint %s. Got %s.", endpoint, client.endpoint)
	}
	if client.client != http.DefaultClient {
		t.Errorf("Expected http.Client %#v. Got %#v.", http.DefaultClient, client.client)
	}
	_, err = NewAPIClient("")
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
	var client APIClient
	for _, tt := range tests {
		client.endpoint = tt.endpoint
		got := client.getURL(tt.path)
		if got != tt.expected {
			t.Errorf("getURL(%q): Got %s. Want %s.", tt.path, got, tt.expected)
		}
	}
}

func TestAPIClientListContainers(t *testing.T) {
	jsonContainers := `[
     {
             "Id": "8dfafdbc3a40",
             "Image": "base:latest",
             "Command": "echo 1",
             "Created": 1367854155,
             "Status": "Exit 0"
     },
     {
             "Id": "9cd87474be90",
             "Image": "base:latest",
             "Command": "echo 222222",
             "Created": 1367854155,
             "Status": "Exit 0"
     },
     {
             "Id": "3176a2479c92",
             "Image": "base:latest",
             "Command": "echo 3333333333333333",
             "Created": 1367854154,
             "Status": "Exit 0"
     },
     {
             "Id": "4cb07b47f9fb",
             "Image": "base:latest",
             "Command": "echo 444444444444444444444444444444444",
             "Created": 1367854152,
             "Status": "Exit 0"
     }
]`
	var expected []ApiContainer
	err := json.Unmarshal([]byte(jsonContainers), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := APIClient{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &FakeRoundTripper{message: jsonContainers, status: http.StatusOK},
		},
	}
	containers, err := client.ListContainers(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(containers, expected) {
		t.Errorf("ListContainers: Expected %#v. Got %#v.", expected, containers)
	}
}

func TestAPIClientListContainersParams(t *testing.T) {
	var tests = []struct {
		input  *ListContainersOptions
		params map[string][]string
	}{
		{nil, map[string][]string{}},
		{&ListContainersOptions{All: true}, map[string][]string{"all": {"1"}}},
		{&ListContainersOptions{All: true, Limit: 10}, map[string][]string{"all": {"1"}, "limit": {"10"}}},
		{
			&ListContainersOptions{All: true, Limit: 10, Since: "adf9983", Before: "abdeef"},
			map[string][]string{"all": {"1"}, "limit": {"10"}, "since": {"adf9983"}, "before": {"abdeef"}},
		},
	}
	fakeRT := FakeRoundTripper{message: "[]", status: http.StatusOK}
	client := APIClient{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &fakeRT,
		},
	}
	for _, tt := range tests {
		client.ListContainers(tt.input)
		got := map[string][]string(fakeRT.requests[0].URL.Query())
		if !reflect.DeepEqual(got, tt.params) {
			t.Errorf("Expected %#v, got %#v.", tt.params, got)
		}
		if path := fakeRT.requests[0].URL.Path; path != "/containers/ps" {
			t.Errorf("Wrong path on request. Want %q. Got %q.", "/containers/ps", path)
		}
		if meth := fakeRT.requests[0].Method; meth != "GET" {
			t.Errorf("Wrong HTTP method. Want GET. Got %s.", meth)
		}
		fakeRT.Reset()
	}
}

func TestAPIClientFailure(t *testing.T) {
	var tests = []struct {
		status  int
		message string
	}{
		{400, "bad parameter"},
		{500, "internal server error"},
	}
	for _, tt := range tests {
		client := APIClient{
			endpoint: "http://localhost:4243",
			client: &http.Client{
				Transport: &FakeRoundTripper{message: tt.message, status: tt.status},
			},
		}
		expected := apiClientError{status: tt.status, message: tt.message}
		containers, err := client.ListContainers(nil)
		if !reflect.DeepEqual(expected, *err.(*apiClientError)) {
			t.Errorf("Wrong error in ListContainers. Want %#v. Got %#v.", expected, err)
		}
		if len(containers) > 0 {
			t.Errorf("ListContainers failure. Expected empty list. Got %#v.", containers)
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
