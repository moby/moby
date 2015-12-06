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
func (i TemplateInspector) Inspect(typedElement interface{}, rawElement []byte) error {
	buffer := new(bytes.Buffer)
	if err := i.tmpl.Execute(buffer, typedElement); err != nil {
		var raw interface{}
		rdr := bytes.NewReader(rawElement)
		dec := json.NewDecoder(rdr)

		if rawErr := dec.Decode(&raw); rawErr != nil {
			return fmt.Errorf("unable to read inspect data: %v\n", rawErr)
		}

		tmplMissingKey := i.tmpl.Option("missingkey=error")
		if rawErr := tmplMissingKey.Execute(buffer, raw); rawErr != nil {
			return fmt.Errorf("Template parsing error: %v\n", err)
		}
	}
	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}

// Flush write the result of inspecting all elements into the output stream.
func (i TemplateInspector) Flush() error {
	_, err := io.Copy(i.outputStream, i.buffer)
	return err
}

// IndentedInspector uses a buffer to stop the indented representation of an element.
type IndentedInspector struct {
	outputStream io.Writer
	indented     *bytes.Buffer
}

// NewIndentedInspector generates a new IndentedInspector.
func NewIndentedInspector(outputStream io.Writer) Inspector {
	indented := new(bytes.Buffer)
	indented.WriteString("[\n")
	return &IndentedInspector{
		outputStream: outputStream,
		indented:     indented,
	}
}

// Inspect writes the raw element with an indented json format.
func (i IndentedInspector) Inspect(_ interface{}, rawElement []byte) error {
	if err := json.Indent(i.indented, rawElement, "", "    "); err != nil {
		return err
	}
	i.indented.WriteByte(',')
	return nil
}

// Flush write the result of inspecting all elements into the output stream.
func (i IndentedInspector) Flush() error {
	if i.indented.Len() > 1 {
		// Remove trailing ','
		i.indented.Truncate(i.indented.Len() - 1)
	}
	i.indented.WriteString("]\n")

	// Note that we will always write "[]" when "-f" isn't specified,
	// to make sure the output would always be array, see
	// https://github.com/docker/docker/pull/9500#issuecomment-65846734
	_, err := io.Copy(i.outputStream, i.indented)
	return err
}
