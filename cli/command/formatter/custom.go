package formatter

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
	FullHeader() interface{}
}

// HeaderContext provides the subContext interface for managing headers
type HeaderContext struct {
	header interface{}
}

// FullHeader returns the header as an interface
func (c *HeaderContext) FullHeader() interface{} {
	return c.header
}

func stripNamePrefix(ss []string) []string {
	sss := make([]string, len(ss))
	for i, s := range ss {
		sss[i] = s[1:]
	}

	return sss
}
