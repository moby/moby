package requestdecorator

import (
	"net/http"
	"strings"
	"testing"
)

func TestUAVersionInfo(t *testing.T) {
	uavi := NewUAVersionInfo("foo", "bar")
	if !uavi.isValid() {
		t.Fatalf("UAVersionInfo should be valid")
	}
	uavi = NewUAVersionInfo("", "bar")
	if uavi.isValid() {
		t.Fatalf("Expected UAVersionInfo to be invalid")
	}
	uavi = NewUAVersionInfo("foo", "")
	if uavi.isValid() {
		t.Fatalf("Expected UAVersionInfo to be invalid")
	}
}

func TestUserAgentDecorator(t *testing.T) {
	httpVersion := make([]UAVersionInfo, 2)
	httpVersion = append(httpVersion, NewUAVersionInfo("testname", "testversion"))
	httpVersion = append(httpVersion, NewUAVersionInfo("name", "version"))
	uad := &UserAgentDecorator{
		Versions: httpVersion,
	}

	req, err := http.NewRequest("GET", "/something", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	reqDecorated, err := uad.ChangeRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if reqDecorated.Header.Get("User-Agent") != "testname/testversion name/version" {
		t.Fatalf("Request should have User-Agent 'testname/testversion name/version'")
	}
}

func TestUserAgentDecoratorErr(t *testing.T) {
	httpVersion := make([]UAVersionInfo, 0)
	uad := &UserAgentDecorator{
		Versions: httpVersion,
	}

	var req *http.Request
	_, err := uad.ChangeRequest(req)
	if err == nil {
		t.Fatalf("Expected to get ErrNilRequest instead no error was returned")
	}
}

func TestMetaHeadersDecorator(t *testing.T) {
	var headers = map[string][]string{
		"key1": {"value1"},
		"key2": {"value2"},
	}
	mhd := &MetaHeadersDecorator{
		Headers: headers,
	}

	req, err := http.NewRequest("GET", "/something", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	reqDecorated, err := mhd.ChangeRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := reqDecorated.Header["key1"]
	if !ok {
		t.Fatalf("Expected to have header key1")
	}
	if v[0] != "value1" {
		t.Fatalf("Expected value for key1 isn't value1")
	}

	v, ok = reqDecorated.Header["key2"]
	if !ok {
		t.Fatalf("Expected to have header key2")
	}
	if v[0] != "value2" {
		t.Fatalf("Expected value for key2 isn't value2")
	}
}

func TestMetaHeadersDecoratorErr(t *testing.T) {
	mhd := &MetaHeadersDecorator{}

	var req *http.Request
	_, err := mhd.ChangeRequest(req)
	if err == nil {
		t.Fatalf("Expected to get ErrNilRequest instead no error was returned")
	}
}

func TestAuthDecorator(t *testing.T) {
	ad := NewAuthDecorator("test", "password")

	req, err := http.NewRequest("GET", "/something", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	reqDecorated, err := ad.ChangeRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	username, password, ok := reqDecorated.BasicAuth()
	if !ok {
		t.Fatalf("Cannot retrieve basic auth info from request")
	}
	if username != "test" {
		t.Fatalf("Expected username to be test, got %s", username)
	}
	if password != "password" {
		t.Fatalf("Expected password to be password, got %s", password)
	}
}

func TestAuthDecoratorErr(t *testing.T) {
	ad := &AuthDecorator{}

	var req *http.Request
	_, err := ad.ChangeRequest(req)
	if err == nil {
		t.Fatalf("Expected to get ErrNilRequest instead no error was returned")
	}
}

func TestRequestFactory(t *testing.T) {
	ad := NewAuthDecorator("test", "password")
	httpVersion := make([]UAVersionInfo, 2)
	httpVersion = append(httpVersion, NewUAVersionInfo("testname", "testversion"))
	httpVersion = append(httpVersion, NewUAVersionInfo("name", "version"))
	uad := &UserAgentDecorator{
		Versions: httpVersion,
	}

	requestFactory := NewRequestFactory(ad, uad)

	if l := len(requestFactory.GetDecorators()); l != 2 {
		t.Fatalf("Expected to have two decorators, got %d", l)
	}

	req, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		t.Fatalf("Cannot retrieve basic auth info from request")
	}
	if username != "test" {
		t.Fatalf("Expected username to be test, got %s", username)
	}
	if password != "password" {
		t.Fatalf("Expected password to be password, got %s", password)
	}
	if req.Header.Get("User-Agent") != "testname/testversion name/version" {
		t.Fatalf("Request should have User-Agent 'testname/testversion name/version'")
	}
}

func TestRequestFactoryNewRequestWithDecorators(t *testing.T) {
	ad := NewAuthDecorator("test", "password")

	requestFactory := NewRequestFactory(ad)

	if l := len(requestFactory.GetDecorators()); l != 1 {
		t.Fatalf("Expected to have one decorators, got %d", l)
	}

	ad2 := NewAuthDecorator("test2", "password2")

	req, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"), ad2)
	if err != nil {
		t.Fatal(err)
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		t.Fatalf("Cannot retrieve basic auth info from request")
	}
	if username != "test2" {
		t.Fatalf("Expected username to be test, got %s", username)
	}
	if password != "password2" {
		t.Fatalf("Expected password to be password, got %s", password)
	}
}

func TestRequestFactoryAddDecorator(t *testing.T) {
	requestFactory := NewRequestFactory()

	if l := len(requestFactory.GetDecorators()); l != 0 {
		t.Fatalf("Expected to have zero decorators, got %d", l)
	}

	ad := NewAuthDecorator("test", "password")
	requestFactory.AddDecorator(ad)

	if l := len(requestFactory.GetDecorators()); l != 1 {
		t.Fatalf("Expected to have one decorators, got %d", l)
	}
}

func TestRequestFactoryNil(t *testing.T) {
	var requestFactory RequestFactory
	_, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("Expected not to get and error, got %s", err)
	}
}
