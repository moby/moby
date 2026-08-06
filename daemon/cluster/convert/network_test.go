package convert

import (
	"testing"
	"time"

	swarmapi "github.com/moby/swarmkit/v2/api"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNetworkConvertBasicNetworkFromGRPCCreatedAt(t *testing.T) {
	expected, err := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 10, 2018 at 7:54pm (PST)")
	if err != nil {
		t.Fatal(err)
	}
	createdAt := timestamppb.New(expected)

	nw := swarmapi.Network{
		Meta: &swarmapi.Meta{
			Version: &swarmapi.Version{
				Index: 1,
			},
			CreatedAt: createdAt,
		},
		Spec: &swarmapi.NetworkSpec{
			Annotations: &swarmapi.Annotations{},
		},
	}

	n := BasicNetworkFromGRPC(&nw)
	if !n.Created.Equal(expected) {
		t.Fatalf("expected time %s; received %s", expected, n.Created)
	}
}
