package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// VolumeConstructor defines a volume constructor function
type VolumeConstructor func(*client.VolumeCreateOptions)

func (d *Daemon) createVolumeWithOptions(ctx context.Context, t testing.TB, f ...VolumeConstructor) (string, error) {
	t.Helper()
	var opts client.VolumeCreateOptions
	for _, fn := range f {
		fn(&opts)
	}

	cli := d.NewClientT(t)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	v, err := cli.VolumeCreate(ctx, opts)
	return v.Volume.Name, err
}

// CreateVolume creates a volume given the specified volume constructor
func (d *Daemon) CreateVolume(ctx context.Context, t testing.TB, f ...VolumeConstructor) (string, error) {
	t.Helper()
	return d.createVolumeWithOptions(ctx, t, f...)
}

// GetVolume returns the swarm volume corresponding to the specified id
func (d *Daemon) GetVolume(ctx context.Context, t testing.TB, id string) *client.VolumeInspectResult {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	// .VolumeInspectWithRaw(ctx, id)
	volume, err := cli.VolumeInspect(ctx, id, client.VolumeInspectOptions{})
	assert.NilError(t, err)
	return &volume
}
