package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/volume"
	"gotest.tools/v3/assert"
)

// VolumeConstructor defines a volume constructor function
type VolumeConstructor func(*volume.CreateOptions)

func (d *Daemon) createVolumeWithOptions(ctx context.Context, t testing.TB, f ...VolumeConstructor) (string, error) {
	t.Helper()
	var opts volume.CreateOptions
	for _, fn := range f {
		fn(&opts)
	}

	cli := d.NewClientT(t)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	v, err := cli.VolumeCreate(ctx, opts)
	return v.Name, err
}

// CreateVolume creates a volume given the specified volume constructor
func (d *Daemon) CreateVolume(ctx context.Context, t testing.TB, f ...VolumeConstructor) (string, error) {
	t.Helper()
	return d.createVolumeWithOptions(ctx, t, f...)
}

// GetVolume returns the swarm volume corresponding to the specified id
func (d *Daemon) GetVolume(ctx context.Context, t testing.TB, id string) *volume.Volume {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	volume, _, err := cli.VolumeInspectWithRaw(ctx, id)
	assert.NilError(t, err)
	return &volume
}
