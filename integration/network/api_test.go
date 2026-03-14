package network

import (
	"context"
	"net/http"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/integration/internal/requirement"
)

func TestAPICreateDeletePredefinedNetworks(t *testing.T) {
	ctx := setupTest(t)
	requirement.TestRequires(t, DaemonIsLinux, SwarmInactive)
	createDeletePredefinedNetwork(t, ctx, "bridge")
	createDeletePredefinedNetwork(t, ctx, "none")
	createDeletePredefinedNetwork(t, ctx, "host")
}

func createDeletePredefinedNetwork(t *testing.T, ctx context.Context, name string) {
	// Create pre-defined network
	config := network.CreateRequest{Name: name}
	expectedStatus := http.StatusForbidden
	CreateNetwork(t, ctx, config, expectedStatus)
	DeleteNetwork(t, ctx, name, false)
}
