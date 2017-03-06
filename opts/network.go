package opts

// NetworkOpts groups options for network drivers
type NetworkOpts struct {
	GroupMapOptions
}

// NewNetworkOpts creates a new NetworOpts
func NewNetworkOpts(validator ValidatorFctType) NetworkOpts {
	return NetworkOpts{NewGroupMapOptions(validator)}
}

// Type returns the type of network option
func (n *NetworkOpts) Type() string {
	return "network"
}
