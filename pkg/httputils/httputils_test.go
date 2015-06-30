package httputils

import (
	"testing"
)

func TestDownload(t *testing.T) {
	_, err := Download("http://docker.com")

	if err != nil {
		t.Errorf("Expected error to not exist when Download(http://docker.com)")
	}

	// Expected status code = 404
	_, err = Download("http://docker.com/abc1234567")
	if err == nil {
		t.Errorf("Expected error to exist when Download(http://docker.com/abc1234567)")
	}
}

func TestNewHTTPRequestError(t *testing.T) {
	errorMessage := "Some error message"
	httpResponse, _ := Download("http://docker.com")
	err := NewHTTPRequestError(errorMessage, httpResponse)

	if err.Error() != errorMessage {
		t.Errorf("Expected err to equal error Message")
	}
}

func TestParseServerHeader(t *testing.T) {
	serverHeader, err := ParseServerHeader("bad header")
	if err.Error() != "Bad header: Failed regex match" {
		t.Errorf("Should fail when header can not be parsed")
	}

	serverHeader, err = ParseServerHeader("(bad header)")
	if err.Error() != "Bad header: '/' missing" {
		t.Errorf("Should fail when header can not be parsed")
	}

	serverHeader, err = ParseServerHeader("(without/spaces)")
	if err.Error() != "Bad header: Expected single space" {
		t.Errorf("Should fail when header can not be parsed")
	}

	serverHeader, err = ParseServerHeader("(header/with space)")
	if serverHeader.App != "(header" {
		t.Errorf("Expected serverHeader.App to equal \"(header\"")
	}

	if serverHeader.Ver != "with" {
		t.Errorf("Expected serverHeader.Ver to equal \"with\"")
	}

	if serverHeader.OS != "header/with space" {
		t.Errorf("Expected serverHeader.OS to equal \"header/with space\"")
	}
}
