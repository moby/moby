package formatter

import (
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/stringutils"
	"strconv"
	"strings"
)

const (
	defaultSearchTableFormat = "table {{.Name}}\t{{.Description}}\t{{.StarCount}}\t{{.IsOfficial}}\t{{.IsAutomated}}"

	starsHeader     = "STARS"
	officialHeader  = "OFFICIAL"
	automatedHeader = "AUTOMATED"
)

// NewSearchFormat returns a Format for rendering using a network Context
func NewSearchFormat(source string) Format {
	switch source {
	case TableFormatKey:
		return defaultSearchTableFormat
	}
	return Format(source)
}

// SearchWrite writes the context
func SearchWrite(ctx Context, results []registrytypes.SearchResult, auto bool, stars int) error {
	render := func(format func(subContext subContext) error) error {
		for _, result := range results {
			// --automated and -s, --stars are deprecated since Docker 1.12
			if (auto && !result.IsAutomated) || (stars > result.StarCount) {
				continue
			}
			searchCtx := &searchContext{trunc: ctx.Trunc, s: result}
			if err := format(searchCtx); err != nil {
				return err
			}
		}
		return nil
	}
	searchCtx := searchContext{}
	searchCtx.header = map[string]string{
		"Name":        nameHeader,
		"Description": descriptionHeader,
		"StarCount":   starsHeader,
		"IsOfficial":  officialHeader,
		"IsAutomated": automatedHeader,
	}
	return ctx.Write(&searchCtx, render)
}

type searchContext struct {
	HeaderContext
	trunc bool
	json  bool
	s     registrytypes.SearchResult
}

func (c *searchContext) MarshalJSON() ([]byte, error) {
	c.json = true
	return marshalJSON(c)
}

func (c *searchContext) Name() string {
	return c.s.Name
}

func (c *searchContext) Description() string {
	desc := strings.Replace(c.s.Description, "\n", " ", -1)
	desc = strings.Replace(desc, "\r", " ", -1)
	if c.trunc {
		desc = stringutils.Ellipsis(desc, 45)
	}
	return desc
}

func (c *searchContext) StarCount() string {
	return strconv.Itoa(c.s.StarCount)
}

func (c *searchContext) IsOfficial() string {
	official := ""
	if c.s.IsOfficial {
		if c.json {
			official = "true"
		} else {
			official = "[OK]"
		}
	} else {
		if c.json {
			official = "false"
		}
	}
	return official
}

func (c *searchContext) IsAutomated() string {
	auto := ""
	if c.s.IsAutomated {
		if c.json {
			auto = "true"
		} else {
			auto = "[OK]"
		}
	} else {
		if c.json {
			auto = "false"
		}
	}
	return auto
}
