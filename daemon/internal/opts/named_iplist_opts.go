package opts

import (
	"fmt"
	"net/netip"
)

// NamedIPListOpts appends to an underlying []netip.Addr.
type NamedIPListOpts struct {
	name string
	ips  *[]netip.Addr
}

// NewNamedIPListOptsRef constructs a NamedIPListOpts and returns its address.
func NewNamedIPListOptsRef(name string, values *[]netip.Addr) *NamedIPListOpts {
	return &NamedIPListOpts{
		name: name,
		ips:  values,
	}
}

// String returns a string representation of the addresses in the underlying []netip.Addr.
func (o *NamedIPListOpts) String() string {
	if len(*o.ips) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", *o.ips)
}

// Set converts value to a netip.Addr and appends it to the underlying []netip.Addr.
func (o *NamedIPListOpts) Set(value string) error {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return err
	}
	*o.ips = append(*o.ips, ip)
	return nil
}

// Type returns a string name for this Option type
func (o *NamedIPListOpts) Type() string {
	return "list"
}

// Name returns the name of the NamedIPListOpts in the configuration.
func (o *NamedIPListOpts) Name() string {
	return o.name
}
