package parsers // import "github.com/docker/docker/pkg/parsers"

import (
	"reflect"
	"testing"
)

func TestParseUintList(t *testing.T) {
	valids := map[string]map[int]bool{
		"":             {},
		"7":            {7: true},
		"1-6":          {1: true, 2: true, 3: true, 4: true, 5: true, 6: true},
		"0-7":          {0: true, 1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true},
		"0,3-4,7,8-10": {0: true, 3: true, 4: true, 7: true, 8: true, 9: true, 10: true},
		"0-0,0,1-4":    {0: true, 1: true, 2: true, 3: true, 4: true},
		"03,1-3":       {1: true, 2: true, 3: true},
		"3,2,1":        {1: true, 2: true, 3: true},
		"0-2,3,1":      {0: true, 1: true, 2: true, 3: true},
	}
	for k, v := range valids {
		out, err := parseUintList(k, 0)
		if err != nil {
			t.Fatalf("Expected not to fail, got %v", err)
		}
		if !reflect.DeepEqual(out, v) {
			t.Fatalf("Expected %v, got %v", v, out)
		}
	}

	invalids := []string{
		"this",
		"1--",
		"1-10,,10",
		"10-1",
		"-1",
		"-1,0",
	}
	for _, v := range invalids {
		if out, err := parseUintList(v, 0); err == nil {
			t.Fatalf("Expected failure with %s but got %v", v, out)
		}
	}
}

func TestParseUintListMaximumLimits(t *testing.T) {
	v := "10,1000"
	if _, err := parseUintList(v, 0); err != nil {
		t.Fatalf("Expected not to fail, got %v", err)
	}
	if _, err := parseUintList(v, 1000); err != nil {
		t.Fatalf("Expected not to fail, got %v", err)
	}
	if out, err := parseUintList(v, 100); err == nil {
		t.Fatalf("Expected failure with %s but got %v", v, out)
	}
}
