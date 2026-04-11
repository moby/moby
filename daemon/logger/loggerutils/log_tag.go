package loggerutils

import (
	"bytes"

	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/logger/templates"
)

// DefaultTemplate defines the defaults template logger should use.
const DefaultTemplate = "{{.ID}}"

const (
	ctrShortID = "{{.ID}}"
	ctrFullID  = "{{.FullID}}"
	ctrName    = "{{.Name}}"
	ctrCommand = "{{.Command}}"
	imgShortID = "{{.ImageID}}"
	imgFullID  = "{{.ImageFullID}}"
	imgName    = "{{.ImageName}}"
	hostName   = "{{.Hostname}}"
)

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
