package service

import (
	"fmt"
	"net"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
)

const (
	defaultSwarmPort = 2477
)

// #29885
func TestSwarmErrorHandling(t *testing.T) {
	ctx := setupTest(t)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", defaultSwarmPort))
	assert.NilError(t, err)
	defer ln.Close()

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	_, err = apiClient.SwarmInit(ctx, client.SwarmInitOptions{
		ListenAddr: d.SwarmListenAddr(),
	})
	assert.ErrorContains(t, err, "address already in use")

}
