package registry

const (
	// DefaultNamespace is the default namespace
	DefaultNamespace = "docker.io"
	// DefaultRegistryVersionHeader is the name of the default HTTP header
	// that carries Registry version info
	DefaultRegistryVersionHeader = "Docker-Distribution-Api-Version"
	// DefaultV1Registry is the URI of the default v1 registry
	DefaultV1Registry = "https://index.docker.io"

	// CertsDir is the directory where certificates are stored
	CertsDir = "/etc/docker/certs.d"

	// IndexServer is the v1 registry server used for user auth + account creation
	IndexServer = DefaultV1Registry + "/v1/"
	// IndexName is the name of the index
	IndexName = "docker.io"

	// NotaryServer is the endpoint serving the Notary trust server
	NotaryServer = "https://notary.docker.io"

	// IndexServer = "https://registry-stage.hub.docker.com/v1/"
)
