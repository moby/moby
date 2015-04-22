package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestInfoApi(t *testing.T) {
	endpoint := "/info"

	statusCode, body, err := sockRequest("GET", endpoint, nil)
	if err != nil || statusCode != http.StatusOK {
		t.Fatalf("Expected %d from info request, got %d", http.StatusOK, statusCode)
	}

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"Images",
		"ExecutionDriver",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"MemTotal",
		"KernelVersion",
		"Driver"}

	out := string(body)
	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}
}
