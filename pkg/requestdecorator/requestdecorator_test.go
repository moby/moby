package requestdecorator

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// The following 2 functions are here for 1.3.3 support
// After we drop 1.3.3 support we can use the functions supported
// in go v1.4.0 +
// BasicAuth returns the username and password provided in the request's
// Authorization header, if the request uses HTTP Basic Authentication.
// See RFC 2617, Section 2.
func basicAuth(r *http.Request) (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}
	return parseBasicAuth(auth)
}

// parseBasicAuth parses an HTTP Basic Authentication string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

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

	username, password, ok := basicAuth(reqDecorated)
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

	if dlen := requestFactory.GetDecorators(); len(dlen) != 2 {
		t.Fatalf("Expected to have two decorators, got %d", dlen)
	}

	req, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}

	username, password, ok := basicAuth(req)
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

	if dlen := requestFactory.GetDecorators(); len(dlen) != 1 {
		t.Fatalf("Expected to have one decorators, got %d", dlen)
	}

	ad2 := NewAuthDecorator("test2", "password2")

	req, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"), ad2)
	if err != nil {
		t.Fatal(err)
	}

	username, password, ok := basicAuth(req)
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

	if dlen := requestFactory.GetDecorators(); len(dlen) != 0 {
		t.Fatalf("Expected to have zero decorators, got %d", dlen)
	}

	ad := NewAuthDecorator("test", "password")
	requestFactory.AddDecorator(ad)

	if dlen := requestFactory.GetDecorators(); len(dlen) != 1 {
		t.Fatalf("Expected to have one decorators, got %d", dlen)
	}
}

func TestRequestFactoryNil(t *testing.T) {
	var requestFactory RequestFactory
	_, err := requestFactory.NewRequest("GET", "/test", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("Expected not to get and error, got %s", err)
	}
}
