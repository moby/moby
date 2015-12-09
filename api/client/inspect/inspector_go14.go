// +build !go1.5

package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// tryeRawInspectFallback executes the inspect template with a raw interface.
// This allows docker cli to parse inspect structs injected with Swarm fields.
// Unfortunately, go 1.4 doesn't fail executing invalid templates when the input is an interface.
// It doesn't allow to modify this behavior either, sending <no value> messages to the output.
// We assume that the template is invalid when there is a <no value>, if the template was valid
// we'd get <nil> or "" values. In that case we fail with the original error raised executing the
// template with the typed input.
func (i *TemplateInspector) tryRawInspectFallback(rawElement []byte, originalErr error) error {
	var raw interface{}
	buffer := new(bytes.Buffer)
	rdr := bytes.NewReader(rawElement)
	dec := json.NewDecoder(rdr)

	if rawErr := dec.Decode(&raw); rawErr != nil {
		return fmt.Errorf("unable to read inspect data: %v", rawErr)
	}

	if rawErr := i.tmpl.Execute(buffer, raw); rawErr != nil {
		return fmt.Errorf("Template parsing error: %v", rawErr)
	}

	if strings.Contains(buffer.String(), "<no value>") {
		return fmt.Errorf("Template parsing error: %v", originalErr)
	}

	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}
