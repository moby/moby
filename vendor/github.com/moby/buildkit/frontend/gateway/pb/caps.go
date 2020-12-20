package moby_buildkit_v1_frontend //nolint:golint

import "github.com/moby/buildkit/util/apicaps"

var Caps apicaps.CapList

// Every backwards or forwards non-compatible change needs to add a new capability row.
// By default new capabilities should be experimental. After merge a capability is
// considered immutable. After a capability is marked stable it should not be disabled.

const (
	CapSolveBase               apicaps.CapID = "solve.base"
	CapSolveInlineReturn       apicaps.CapID = "solve.inlinereturn"
	CapResolveImage            apicaps.CapID = "resolveimage"
	CapResolveImageResolveMode apicaps.CapID = "resolveimage.resolvemode"
	CapReadFile                apicaps.CapID = "readfile"
	CapReturnResult            apicaps.CapID = "return"
	CapReturnMap               apicaps.CapID = "returnmap"
	CapReadDir                 apicaps.CapID = "readdir"
	CapStatFile                apicaps.CapID = "statfile"
	CapImportCaches            apicaps.CapID = "importcaches"

	// CapProtoRefArray is a capability to return arrays of refs instead of single
	// refs. This capability is only for the wire format change and shouldn't be
	// used in frontends for feature detection.
	CapProtoRefArray apicaps.CapID = "proto.refarray"

	// CapReferenceOutput is a capability to use a reference of a solved result as
	// an llb.Output.
	CapReferenceOutput apicaps.CapID = "reference.output"

	// CapFrontendInputs is a capability to request frontend inputs from the
	// LLBBridge GRPC server.
	CapFrontendInputs apicaps.CapID = "frontend.inputs"

	// CapGatewaySolveMetadata can be used to check if solve calls from gateway reliably return metadata
	CapGatewaySolveMetadata apicaps.CapID = "gateway.solve.metadata"

	// CapGatewayExec is the capability to create and interact with new
	// containers directly through the gateway
	CapGatewayExec apicaps.CapID = "gateway.exec"

	// CapFrontendCaps can be used to check that frontends define support for certain capabilities
	CapFrontendCaps apicaps.CapID = "frontend.caps"

	// CapGatewayEvaluateSolve is a capability to immediately unlazy solve
	// results. This is generally used by the client to return and handle solve
	// errors.
	CapGatewayEvaluateSolve apicaps.CapID = "gateway.solve.evaluate"
)

func init() {

	Caps.Init(apicaps.Cap{
		ID:      CapSolveBase,
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:         CapSolveInlineReturn,
		Name:       "inline return from solve",
		Enabled:    true,
		Deprecated: true,
		Status:     apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapResolveImage,
		Name:    "resolve remote image config",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapResolveImageResolveMode,
		Name:    "resolve remote image config with custom resolvemode",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapReadFile,
		Name:    "read static file",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapReturnResult,
		Name:    "return solve result",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapReturnMap,
		Name:    "return reference map",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapReadDir,
		Name:    "read static directory",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapStatFile,
		Name:    "stat a file",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapImportCaches,
		Name:    "import caches",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapProtoRefArray,
		Name:    "wire format ref arrays",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapReferenceOutput,
		Name:    "reference output",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapFrontendInputs,
		Name:    "frontend inputs",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapGatewaySolveMetadata,
		Name:    "gateway metadata",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapGatewayExec,
		Name:    "gateway exec",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapFrontendCaps,
		Name:    "frontend capabilities",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})

	Caps.Init(apicaps.Cap{
		ID:      CapGatewayEvaluateSolve,
		Name:    "gateway evaluate solve",
		Enabled: true,
		Status:  apicaps.CapStatusExperimental,
	})
}
