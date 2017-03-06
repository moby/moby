package opts

// IPAMOpts groups options for IPAM driver
type IPAMOpts struct {
	GroupMapOptions
}

// NewIPAMOpts creates a new IPAMOpts
func NewIPAMOpts(validator ValidatorFctType) IPAMOpts {
	return IPAMOpts{NewGroupMapOptions(validator)}
}

// Type returns the type of IPAM option
func (n *IPAMOpts) Type() string {
	return "ipam"
}
