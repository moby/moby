package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/template"
)

// Inspector defines an interface to implement to process elements
type Inspector interface {
	Inspect(typedElement interface{}, rawElement []byte) error
	Flush() error
}

// TemplateInspector uses a text template to inspect elements.
type TemplateInspector struct {
	outputStream io.Writer
	buffer       *bytes.Buffer
	tmpl         *template.Template
}

// NewTemplateInspector creates a new inspector with a template.
func NewTemplateInspector(outputStream io.Writer, tmpl *template.Template) Inspector {
	return &TemplateInspector{
		outputStream: outputStream,
		buffer:       new(bytes.Buffer),
		tmpl:         tmpl,
	}
}

// Inspect executes the inspect template.
// It decodes the raw element into a map if the initial execution fails.
// This allows docker cli to parse inspect structs injected with Swarm fields.
func (i *TemplateInspector) Inspect(typedElement interface{}, rawElement []byte) error {
	buffer := new(bytes.Buffer)
	if err := i.tmpl.Execute(buffer, typedElement); err != nil {
		if rawElement == nil {
			return fmt.Errorf("Template parsing error: %v", err)
		}
		return i.tryRawInspectFallback(rawElement)
	}
	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}

func (i *TemplateInspector) tryRawInspectFallback(rawElement []byte) error {
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

// Flush write the result of inspecting all elements into the output stream.
func (i *TemplateInspector) Flush() error {
	_, err := io.Copy(i.outputStream, i.buffer)
	return err
}

// IndentedInspector uses a buffer to stop the indented representation of an element.
type IndentedInspector struct {
	outputStream io.Writer
	elements     []interface{}
}

// NewIndentedInspector generates a new IndentedInspector.
func NewIndentedInspector(outputStream io.Writer) Inspector {
	return &IndentedInspector{
		outputStream: outputStream,
	}
}

// Inspect writes the raw element with an indented json format.
func (i *IndentedInspector) Inspect(typedElement interface{}, _ []byte) error {
	i.elements = append(i.elements, typedElement)
	return nil
}

// Flush write the result of inspecting all elements into the output stream.
func (i *IndentedInspector) Flush() error {
	if len(i.elements) == 0 {
		_, err := io.WriteString(i.outputStream, "[]\n")
		return err
	}

	buffer, err := json.MarshalIndent(i.elements, "", "    ")
	if err != nil {
		return err
	}

	if _, err := io.Copy(i.outputStream, bytes.NewReader(buffer)); err != nil {
		return err
	}
	_, err = io.WriteString(i.outputStream, "\n")
	return err
}
