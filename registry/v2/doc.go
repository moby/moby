// Package v2 describes routes, urls and the error codes used in the Docker
// Registry JSON HTTP API V2. In addition to declarations, descriptors are
// provided for routes and error codes that can be used for implementation and
// automatically generating documentation.
//
// Definitions here are considered to be locked down for the V2 registry api.
// Any changes must be considered carefully and should not proceed without a
// change proposal.
//
// Currently, while the HTTP API definitions are considered stable, the Go API
// exports are considered unstable. Go API consumers should take care when
// relying on these definitions until this message is deleted.
package v2
