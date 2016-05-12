package loggerutils

import (
	"bytes"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/utils/templates"
)

// ParseLogTag generates a context aware tag for consistency across different
// log drivers based on the context of the running container.
func ParseLogTag(ctx logger.Context, defaultTemplate string) (string, error) {
	tagTemplate := ctx.Config["tag"]
	if tagTemplate == "" {
		tagTemplate = defaultTemplate
	}

	tmpl, err := templates.NewParse("log-tag", tagTemplate)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, &ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}
