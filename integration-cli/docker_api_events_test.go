package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestEventsApiGetLineDelim(t *testing.T) {
	name := "testimageevents"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        MAINTAINER "docker"`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteImages(name); err != nil {
		t.Fatal(err)
	}

	endpoint := fmt.Sprintf("/events?since=1&until=%d", time.Now().Unix())
	body, err := sockRequest("GET", endpoint)
	if err != nil {
		t.Fatal(err)
	}

	linesRead := 0
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() && linesRead < 2 {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// make sure line delimited json
		res := make(map[string]interface{})
		if err := json.Unmarshal(line, &res); err != nil {
			t.Fatalf("Unmarshaling the line as JSON failed: %v", err)
		}

		linesRead++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner failed: %v", err)
	}

	if linesRead < 2 {
		t.Fatalf("Only %d lines were read from the stream", linesRead)
	}

	logDone("events - test the api response is line delimited json")
}
