package sdk

import "github.com/moby/moby/v2/internal/extensions"

// ProtocolVersion is the startup protocol version spoken by this package.
const ProtocolVersion = 1

// ReadinessAck is written to stdout by an extension once it is listening.
const ReadinessAck = "ready\n"

// StartupConfig is written to an extension binary's stdin at launch. It is JSON
// (not gRPC), since it bootstraps the connection itself.
type StartupConfig struct {
	Endpoint        string `json:"endpoint"`
	ProtocolVersion int    `json:"protocolVersion"`
	// Config is the extension's own configuration, delivered by id just as an
	// in-process extension receives it at Init -- so the same declaration is
	// configured the same way wherever it runs. It is the parsed config object
	// (as from daemon.json), nil when none is configured.
	Config extensions.Config `json:"config,omitempty"`
	// CallbackEndpoint is the unix socket the daemon serves the extension's
	// declared dependencies on. The SDK dials it at Initialize and hands the
	// extension a resolver backed by it, so a dependency call is routed to the
	// real provider. Empty when the host offers no dependencies.
	CallbackEndpoint string `json:"callbackEndpoint,omitempty"`
}
