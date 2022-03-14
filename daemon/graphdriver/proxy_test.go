package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/pkg/idtools"
	"gotest.tools/v3/assert"
)

func TestGraphDriverInitRequestIsCompatible(t *testing.T) {
	// Graph driver plugins may unmarshal into this version of the init
	// request struct. Verify that the serialization of
	// graphDriverInitRequest is fully backwards compatible.

	type graphDriverInitRequestV1 struct {
		Home    string
		Opts    []string        `json:"Opts"`
		UIDMaps []idtools.IDMap `json:"UIDMaps"`
		GIDMaps []idtools.IDMap `json:"GIDMaps"`
	}

	args := graphDriverInitRequest{
		Home: "homedir",
		Opts: []string{"option1", "option2"},
		IdentityMapping: idtools.IdentityMapping{
			UIDMaps: []idtools.IDMap{{ContainerID: 123, HostID: 456, Size: 42}},
			GIDMaps: []idtools.IDMap{{ContainerID: 789, HostID: 1011, Size: 16}},
		},
	}
	v, err := json.Marshal(&args)
	assert.NilError(t, err)

	var got graphDriverInitRequestV1
	assert.NilError(t, json.Unmarshal(v, &got))
	want := graphDriverInitRequestV1{
		Home:    args.Home,
		Opts:    args.Opts,
		UIDMaps: args.UIDMaps,
		GIDMaps: args.GIDMaps,
	}
	assert.DeepEqual(t, got, want)
}
