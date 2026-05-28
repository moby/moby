package marshal

import (
	"bytes"
	"encoding/json"
)

// JSON marshals the object _without_ escaping HTML
func JSON(obj interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(obj)
	return bytes.TrimSpace(buf.Bytes()), err
}
