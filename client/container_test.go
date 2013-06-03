package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

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
	var expected []docker.ApiContainer
	err := json.Unmarshal([]byte(jsonContainers), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := Client{
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
	client := Client{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &fakeRT,
		},
	}
	u, _ := url.Parse(client.getURL("/containers/ps"))
	for _, tt := range tests {
		client.ListContainers(tt.input)
		got := map[string][]string(fakeRT.requests[0].URL.Query())
		if !reflect.DeepEqual(got, tt.params) {
			t.Errorf("Expected %#v, got %#v.", tt.params, got)
		}
		if path := fakeRT.requests[0].URL.Path; path != u.Path {
			t.Errorf("Wrong path on request. Want %q. Got %q.", u.Path, path)
		}
		if meth := fakeRT.requests[0].Method; meth != "GET" {
			t.Errorf("Wrong HTTP method. Want GET. Got %s.", meth)
		}
		fakeRT.Reset()
	}
}

func TestAPIClientListContainersFailure(t *testing.T) {
	var tests = []struct {
		status  int
		message string
	}{
		{400, "bad parameter"},
		{500, "internal server error"},
	}
	for _, tt := range tests {
		client := Client{
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

func TestAPIClientInspectContainer(t *testing.T) {
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
             "Created": "2013-05-07T14:51:42.087658+02:00",
             "Path": "date",
             "Args": [],
             "Config": {
                     "Hostname": "4fa6e0f0c678",
                     "User": "",
                     "Memory": 0,
                     "MemorySwap": 0,
                     "AttachStdin": false,
                     "AttachStdout": true,
                     "AttachStderr": true,
                     "PortSpecs": null,
                     "Tty": false,
                     "OpenStdin": false,
                     "StdinOnce": false,
                     "Env": null,
                     "Cmd": [
                             "date"
                     ],
                     "Dns": null,
                     "Image": "base",
                     "Volumes": {},
                     "VolumesFrom": ""
             },
             "State": {
                     "Running": false,
                     "Pid": 0,
                     "ExitCode": 0,
                     "StartedAt": "2013-05-07T14:51:42.087658+02:00",
                     "Ghost": false
             },
             "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "NetworkSettings": {
                     "IpAddress": "",
                     "IpPrefixLen": 0,
                     "Gateway": "",
                     "Bridge": "",
                     "PortMapping": null
             },
             "SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
             "ResolvConfPath": "/etc/resolv.conf",
             "Volumes": {}
}`
	var expected docker.Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := Client{
		endpoint: "http://localhost:4343",
		client:   &http.Client{Transport: &fakeRT},
	}
	id := "4fa6e0f0c678"
	container, err := client.InspectContainer(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*container, expected) {
		t.Errorf("InspectContainer(%q): Expected %#v. Got %#v.", id, expected, container)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/json"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestInspectContainerFailure(t *testing.T) {
	var tests = []struct {
		status  int
		message string
	}{
		{404, "no such container"},
		{500, "internal server error"},
	}
	for _, tt := range tests {
		client := Client{
			endpoint: "http://localhost:4243",
			client: &http.Client{
				Transport: &FakeRoundTripper{message: tt.message, status: tt.status},
			},
		}
		expected := apiClientError{status: tt.status, message: tt.message}
		container, err := client.InspectContainer("abe033")
		if container != nil {
			t.Errorf("InspectContainer: Expected <nil> container, got %#v", container)
		}
		if !reflect.DeepEqual(expected, *err.(*apiClientError)) {
			t.Errorf("InspectContainer: Wrong error information. Want %#v. Got %#v.", expected, err)
		}
	}
}
