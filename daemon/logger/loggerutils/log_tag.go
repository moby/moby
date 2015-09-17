package loggerutils

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
)

// ParseLogTag generates a context aware tag for consistency across different
// log drivers based on the context of the running container.
func ParseLogTag(ctx logger.Context, defaultTemplate string) (string, error) {
	tagTemplate := lookupTagTemplate(ctx, defaultTemplate)

	tmpl, err := template.New("log-tag").Parse(tagTemplate)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, &ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func lookupTagTemplate(ctx logger.Context, defaultTemplate string) string {
	tagTemplate := ctx.Config["tag"]

	deprecatedConfigs := []string{"syslog-tag", "gelf-tag", "fluentd-tag"}
	for i := 0; tagTemplate == "" && i < len(deprecatedConfigs); i++ {
		cfg := deprecatedConfigs[i]
		if ctx.Config[cfg] != "" {
			tagTemplate = ctx.Config[cfg]
			logrus.Warn(fmt.Sprintf("Using log tag from deprecated log-opt '%s'. Please use: --log-opt tag=\"%s\"", cfg, tagTemplate))
		}
	}

	if tagTemplate == "" {
		tagTemplate = defaultTemplate
	}

	return tagTemplate
}
