package protogen

import (
	"testing"

	createspecv0 "github.com/moby/moby/v2/extpoints/createspec/v0"
	"gotest.tools/v3/assert"
)

// TestSpecRequestRoundTrip guards the Go<->proto conversions against drift, and
// specifically locks down ContainerID: it is ContainerID on the contract but
// container_id / ContainerId in the generated proto, so the two conversion sides
// must bridge the differing names. A value lost to that mismatch -- the exact
// hazard of the clean-snake-case fix -- would not survive the round trip.
func TestSpecRequestRoundTrip(t *testing.T) {
	req := &createspecv0.SpecRequest{
		ContainerID: "abc123",
		Name:        "web",
		Spec:        []byte(`{"ociVersion":"1.0.0"}`),
		Labels:      map[string]string{"k": "v"},
	}

	assert.DeepEqual(t, req, specRequestFromProto(specRequestToProto(req)))
}
