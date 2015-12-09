// +build go1.5

package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func (i *TemplateInspector) tryRawInspectFallback(rawElement []byte, _ error) error {
	var raw interface{}
	buffer := new(bytes.Buffer)
	rdr := bytes.NewReader(rawElement)
	dec := json.NewDecoder(rdr)

	if rawErr := dec.Decode(&raw); rawErr != nil {
		return fmt.Errorf("unable to read inspect data: %v", rawErr)
	}

	tmplMissingKey := i.tmpl.Option("missingkey=error")
	if rawErr := tmplMissingKey.Execute(buffer, raw); rawErr != nil {
		return fmt.Errorf("Template parsing error: %v", rawErr)
	}

	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}
