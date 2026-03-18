package graphdriver

import "testing"

func TestParseKeyValueOpt(t *testing.T) {
	invalids := map[string]string{
		"":    "unable to parse storage-opt key/value: ",
		"key": "unable to parse storage-opt key/value: key",
	}
	for invalid, expectedError := range invalids {
		if _, _, err := ParseStorageOptKeyValue(invalid); err == nil || err.Error() != expectedError {
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
		key, value, err := ParseStorageOptKeyValue(valid)
		if err != nil {
			t.Fatal(err)
		}
		if key != expectedKeyValue[0] || value != expectedKeyValue[1] {
			t.Fatalf("Expected {%v: %v} got {%v: %v}", expectedKeyValue[0], expectedKeyValue[1], key, value)
		}
	}
}
