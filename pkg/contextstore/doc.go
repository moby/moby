// Package contextstore provides a generic way to store credentials to connect to virtually any kind of remote system
// the `context` term comes from the similar feature in Kubernetes kubectl config files.
//
// conceptually, a context is a set of metadata and TLS data, that can be used to connect to various endpoints
// of the remote system. TLS data and metadata are stored separately, so that in the future, we will be able to store sensitive
// information in a more secure way, depending on the os we are running on (e.g.: on Windows we could use the user Certificate Store, on Mac OS the user Keychain...)
//
// current implementation is purely file based with the following structure:
// ${CONTEXT_ROOT}
//   - config.json: contains the current "default" context
//   - meta/
//     - context1/meta.json: contains context medata (key/value pairs) as well as a list of endpoints (themselves containing key/value pair metadata)
//     - contexts/can/also/be/folded/like/this/meta.json: same as context1, but for a context named `contexts/can/also/be/folded/like/this`
//   - tls/
//     - context1/endpoint1/: directory containing TLS data for the endpoint1 in context1
//
// the context store itself has abolutely no knowledge about a docker or a kubernetes endpoint should contain in term of metadata or TLS config
// client code is responsible for generating and parsing endpoint metadata and TLS files
// The multi-endpoint approach of this package allows to combine many different endpoints in the same "context" (e.g., the Docker CLI
// will be able for a single context to define both a docker endpoint and a Kubernetes endpoint for the same cluster, and also specify which
// orchestrator to use by default when deploying a compose stack on this cluster)
package contextstore
