package label

// Pre-defined label keys similar to BuildKit ones
// https://github.com/moby/buildkit/blob/v0.11.6/worker/label/label.go#L3-L16
const (
	prefix = "org.mobyproject.buildkit.worker.moby."

	HostGatewayIP = prefix + "host-gateway-ip"
)
