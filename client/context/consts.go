package context

const (
	// DockerContextEnvVar is the environment variable used to override default context
	DockerContextEnvVar = "DOCKER_CONTEXT"

	// internal keys for endpoint metadata and TLS data
	hostKey           = "host"
	apiVersionKey     = "apiVersion"
	skipTLSVerifyKey  = "skipTLSVerify"
	dockerEndpointKey = "docker"
	caKey             = "ca.pem"
	certKey           = "cert.pem"
	keyKey            = "key.pem"
)
