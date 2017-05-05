package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/swarm"
	units "github.com/docker/go-units"
)

const (
	defaultSecretTableFormat = "table {{.ID}}\t{{.Name}}\t{{.CreatedAt}}\t{{.UpdatedAt}}"
	secretIDHeader           = "ID"
	secretNameHeader         = "NAME"
	secretCreatedHeader      = "CREATED"
	secretUpdatedHeader      = "UPDATED"
)

// NewSecretFormat returns a Format for rendering using a network Context
func NewSecretFormat(source string, quiet bool) Format {
	switch source {
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		return defaultSecretTableFormat
	}
	return Format(source)
}

// SecretWrite writes the context
func SecretWrite(ctx Context, secrets []swarm.Secret) error {
	render := func(format func(subContext subContext) error) error {
		for _, secret := range secrets {
			secretCtx := &secretContext{s: secret}
			if err := format(secretCtx); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(newSecretContext(), render)
}

func newSecretContext() *secretContext {
	sCtx := &secretContext{}

	sCtx.header = map[string]string{
		"ID":        secretIDHeader,
		"Name":      nameHeader,
		"CreatedAt": secretCreatedHeader,
		"UpdatedAt": secretUpdatedHeader,
		"Labels":    labelsHeader,
	}
	return sCtx
}

type secretContext struct {
	HeaderContext
	s swarm.Secret
}

func (c *secretContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *secretContext) ID() string {
	return c.s.ID
}

func (c *secretContext) Name() string {
	return c.s.Spec.Annotations.Name
}

func (c *secretContext) CreatedAt() string {
	return units.HumanDuration(time.Now().UTC().Sub(c.s.Meta.CreatedAt)) + " ago"
}

func (c *secretContext) UpdatedAt() string {
	return units.HumanDuration(time.Now().UTC().Sub(c.s.Meta.UpdatedAt)) + " ago"
}

func (c *secretContext) Labels() string {
	mapLabels := c.s.Spec.Annotations.Labels
	if mapLabels == nil {
		return ""
	}
	var joinLabels []string
	for k, v := range mapLabels {
		joinLabels = append(joinLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(joinLabels, ",")
}

func (c *secretContext) Label(name string) string {
	if c.s.Spec.Annotations.Labels == nil {
		return ""
	}
	return c.s.Spec.Annotations.Labels[name]
}
