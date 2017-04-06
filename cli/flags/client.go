package flags

// ClientOptions are the options used to configure the client cli
type ClientOptions struct {
	Common    *CommonOptions
	ConfigDir string
	Version   bool
}

// NewClientOptions returns a new ClientOptions
func NewClientOptions() *ClientOptions {
	return &ClientOptions{Common: NewCommonOptions()}
}
