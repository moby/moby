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

	// Fast-path for common / basic templates.
	switch tagTemplate {
	case "":
		return "", nil
	case ctrShortID:
		return info.ID(), nil
	case ctrFullID:
		return info.FullID(), nil
	case ctrName:
		return info.Name(), nil
	case ctrCommand:
		return info.Command(), nil
	case imgShortID:
		return info.ImageID(), nil
	case imgFullID:
		return info.ImageFullID(), nil
	case imgName:
		return info.ImageName(), nil
	case hostName:
		return info.Hostname()
	default:
		tmpl, err := templates.NewParse("log-tag", tagTemplate)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, &info); err != nil {
			return "", err
		}

		return buf.String(), nil
	}
}
