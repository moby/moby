package formatter

import (
	"strings"
)

const (
	tableKey = "table"

	imageHeader        = "IMAGE"
	createdSinceHeader = "CREATED"
	createdAtHeader    = "CREATED AT"
	sizeHeader         = "SIZE"
	labelsHeader       = "LABELS"
	nameHeader         = "NAME"
	driverHeader       = "DRIVER"
	scopeHeader        = "SCOPE"
)

type subContext interface {
	fullHeader() string
	addHeader(header string)
}

type baseSubContext struct {
	header []string
}

func (c *baseSubContext) fullHeader() string {
	if c.header == nil {
		return ""
	}
	return strings.Join(c.header, "\t")
}

func (c *baseSubContext) addHeader(header string) {
	if c.header == nil {
		c.header = []string{}
	}
	c.header = append(c.header, strings.ToUpper(header))
}

func stripNamePrefix(ss []string) []string {
	sss := make([]string, len(ss))
	for i, s := range ss {
		sss[i] = s[1:]
	}

	return sss
}
