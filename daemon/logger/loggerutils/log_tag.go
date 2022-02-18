package loggerutils // import "github.com/moby/moby/daemon/logger/loggerutils"

import (
	"bytes"

	"github.com/moby/moby/daemon/logger"
	"github.com/moby/moby/daemon/logger/templates"
)

// DefaultTemplate defines the defaults template logger should use.
const DefaultTemplate = "{{.ID}}"

// ParseLogTag generates a context aware tag for consistency across different
// log drivers based on the context of the running container.
func ParseLogTag(info logger.Info, defaultTemplate string) (string, error) {
	tagTemplate := info.Config["tag"]
	if tagTemplate == "" {
		tagTemplate = defaultTemplate
	}

	tmpl, err := templates.NewParse("log-tag", tagTemplate)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, &info); err != nil {
		return "", err
	}

	return buf.String(), nil
}
