package inspect

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/templates"
	"github.com/pkg/errors"
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

// NewTemplateInspectorFromString creates a new TemplateInspector from a string
// which is compiled into a template.
func NewTemplateInspectorFromString(out io.Writer, tmplStr string) (Inspector, error) {
	if tmplStr == "" {
		return NewIndentedInspector(out), nil
	}

	tmpl, err := templates.Parse(tmplStr)
	if err != nil {
		return nil, errors.Errorf("Template parsing error: %s", err)
	}
	return NewTemplateInspector(out, tmpl), nil
}

// GetRefFunc is a function which used by Inspect to fetch an object from a
// reference
type GetRefFunc func(ref string) (interface{}, []byte, error)

// Inspect fetches objects by reference using GetRefFunc and writes the json
// representation to the output writer.
func Inspect(out io.Writer, references []string, tmplStr string, getRef GetRefFunc) error {
	inspector, err := NewTemplateInspectorFromString(out, tmplStr)
	if err != nil {
		return cli.StatusError{StatusCode: 64, Status: err.Error()}
	}

	var inspectErrs []string
	for _, ref := range references {
		element, raw, err := getRef(ref)
		if err != nil {
			inspectErrs = append(inspectErrs, err.Error())
			continue
		}

		if err := inspector.Inspect(element, raw); err != nil {
			inspectErrs = append(inspectErrs, err.Error())
		}
	}

	if err := inspector.Flush(); err != nil {
		logrus.Errorf("%s\n", err)
	}

	if len(inspectErrs) != 0 {
		return cli.StatusError{
			StatusCode: 1,
			Status:     strings.Join(inspectErrs, "\n"),
		}
	}
	return nil
}

// Inspect executes the inspect template.
// It decodes the raw element into a map if the initial execution fails.
// This allows docker cli to parse inspect structs injected with Swarm fields.
func (i *TemplateInspector) Inspect(typedElement interface{}, rawElement []byte) error {
	buffer := new(bytes.Buffer)
	if err := i.tmpl.Execute(buffer, typedElement); err != nil {
		if rawElement == nil {
			return errors.Errorf("Template parsing error: %v", err)
		}
		return i.tryRawInspectFallback(rawElement)
	}
	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}

// tryRawInspectFallback executes the inspect template with a raw interface.
// This allows docker cli to parse inspect structs injected with Swarm fields.
func (i *TemplateInspector) tryRawInspectFallback(rawElement []byte) error {
	var raw interface{}
	buffer := new(bytes.Buffer)
	rdr := bytes.NewReader(rawElement)
	dec := json.NewDecoder(rdr)

	if rawErr := dec.Decode(&raw); rawErr != nil {
		return errors.Errorf("unable to read inspect data: %v", rawErr)
	}

	tmplMissingKey := i.tmpl.Option("missingkey=error")
	if rawErr := tmplMissingKey.Execute(buffer, raw); rawErr != nil {
		return errors.Errorf("Template parsing error: %v", rawErr)
	}

	i.buffer.Write(buffer.Bytes())
	i.buffer.WriteByte('\n')
	return nil
}

// Flush writes the result of inspecting all elements into the output stream.
func (i *TemplateInspector) Flush() error {
	if i.buffer.Len() == 0 {
		_, err := io.WriteString(i.outputStream, "\n")
		return err
	}
	_, err := io.Copy(i.outputStream, i.buffer)
	return err
}

// IndentedInspector uses a buffer to stop the indented representation of an element.
type IndentedInspector struct {
	outputStream io.Writer
	elements     []interface{}
	rawElements  [][]byte
}

// NewIndentedInspector generates a new IndentedInspector.
func NewIndentedInspector(outputStream io.Writer) Inspector {
	return &IndentedInspector{
		outputStream: outputStream,
	}
}

// Inspect writes the raw element with an indented json format.
func (i *IndentedInspector) Inspect(typedElement interface{}, rawElement []byte) error {
	if rawElement != nil {
		i.rawElements = append(i.rawElements, rawElement)
	} else {
		i.elements = append(i.elements, typedElement)
	}
	return nil
}

// Flush writes the result of inspecting all elements into the output stream.
func (i *IndentedInspector) Flush() error {
	if len(i.elements) == 0 && len(i.rawElements) == 0 {
		_, err := io.WriteString(i.outputStream, "[]\n")
		return err
	}

	var buffer io.Reader
	if len(i.rawElements) > 0 {
		bytesBuffer := new(bytes.Buffer)
		bytesBuffer.WriteString("[")
		for idx, r := range i.rawElements {
			bytesBuffer.Write(r)
			if idx < len(i.rawElements)-1 {
				bytesBuffer.WriteString(",")
			}
		}
		bytesBuffer.WriteString("]")
		indented := new(bytes.Buffer)
		if err := json.Indent(indented, bytesBuffer.Bytes(), "", "    "); err != nil {
			return err
		}
		buffer = indented
	} else {
		b, err := json.MarshalIndent(i.elements, "", "    ")
		if err != nil {
			return err
		}
		buffer = bytes.NewReader(b)
	}

	if _, err := io.Copy(i.outputStream, buffer); err != nil {
		return err
	}
	_, err := io.WriteString(i.outputStream, "\n")
	return err
}
