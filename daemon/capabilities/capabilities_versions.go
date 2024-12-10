package capabilities

// CapabilitiesV1 must implement Capabilities
var _ VersionedCapabilities = &capabilitiesV1{}

type capabilitiesV1 struct {
	CapabilitiesBase
	RegistryClientAuth bool `json:"registry-client-auth"`
}
