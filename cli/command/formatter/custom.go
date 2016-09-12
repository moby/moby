package formatter

import (
	"strings"
)

const (
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
	FullHeader() string
	AddHeader(header string)
}

// HeaderContext provides the subContext interface for managing headers
type HeaderContext struct {
	header []string
}

// FullHeader returns the header as a string
func (c *HeaderContext) FullHeader() string {
	if c.header == nil {
		return ""
	}
	return strings.Join(c.header, "\t")
}

// AddHeader adds another column to the header
func (c *HeaderContext) AddHeader(header string) {
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
