package parsers

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseKeyValueOpt(t *testing.T) {
	invalids := map[string]string{
		"":    "Unable to parse key/value option: ",
		"key": "Unable to parse key/value option: key",
	}
	for invalid, expectedError := range invalids {
		if _, _, err := ParseKeyValueOpt(invalid); err == nil || err.Error() != expectedError {
			t.Fatalf("Expected error %v for %v, got %v", expectedError, invalid, err)
		}
	}
	valids := map[string][]string{
		"key=value":               {"key", "value"},
		" key = value ":           {"key", "value"},
		"key=value1=value2":       {"key", "value1=value2"},
		" key = value1 = value2 ": {"key", "value1 = value2"},
	}
	for valid, expectedKeyValue := range valids {
		key, value, err := ParseKeyValueOpt(valid)
		if err != nil {
			t.Fatal(err)
		}
		if key != expectedKeyValue[0] || value != expectedKeyValue[1] {
			t.Fatalf("Expected {%v: %v} got {%v: %v}", expectedKeyValue[0], expectedKeyValue[1], key, value)
		}
	}
}

func TestParsePortRange(t *testing.T) {
	if start, end, err := ParsePortRange("8000-8080"); err != nil || start != 8000 || end != 8080 {
		t.Fatalf("Error: %s or Expecting {start,end} values {8000,8080} but found {%d,%d}.", err, start, end)
	}
}

func TestParsePortRangeEmpty(t *testing.T) {
	if _, _, err := ParsePortRange(""); err == nil || err.Error() != "Empty string specified for ports." {
		t.Fatalf("Expected error 'Empty string specified for ports.', got %v", err)
	}
}

func TestParsePortRangeWithNoRange(t *testing.T) {
	start, end, err := ParsePortRange("8080")
	if err != nil {
		t.Fatal(err)
	}
	if start != 8080 || end != 8080 {
		t.Fatalf("Expected start and end to be the same and equal to 8080, but were %v and %v", start, end)
	}
}

func TestParsePortRangeIncorrectRange(t *testing.T) {
	if _, _, err := ParsePortRange("9000-8080"); err == nil || !strings.Contains(err.Error(), "Invalid range specified for the Port") {
		t.Fatalf("Expecting error 'Invalid range specified for the Port' but received %s.", err)
	}
}

func TestParsePortRangeIncorrectEndRange(t *testing.T) {
	if _, _, err := ParsePortRange("8000-a"); err == nil || !strings.Contains(err.Error(), "invalid syntax") {
		t.Fatalf("Expecting error 'Invalid range specified for the Port' but received %s.", err)
	}

	if _, _, err := ParsePortRange("8000-30a"); err == nil || !strings.Contains(err.Error(), "invalid syntax") {
		t.Fatalf("Expecting error 'Invalid range specified for the Port' but received %s.", err)
	}
}

func TestParsePortRangeIncorrectStartRange(t *testing.T) {
	if _, _, err := ParsePortRange("a-8000"); err == nil || !strings.Contains(err.Error(), "invalid syntax") {
		t.Fatalf("Expecting error 'Invalid range specified for the Port' but received %s.", err)
	}

	if _, _, err := ParsePortRange("30a-8000"); err == nil || !strings.Contains(err.Error(), "invalid syntax") {
		t.Fatalf("Expecting error 'Invalid range specified for the Port' but received %s.", err)
	}
}

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
		out, err := ParseUintList(k)
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
		if out, err := ParseUintList(v); err == nil {
			t.Fatalf("Expected failure with %s but got %v", v, out)
		}
	}
}
