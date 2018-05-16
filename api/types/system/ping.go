package system

// Ping contains response of Engine API:
// GET "/_ping"
type Ping struct {
	APIVersion   string
	OSType       string
	Experimental bool
}
