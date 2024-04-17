package csi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moby/swarmkit/v2/agent/csi/plugin"
	"github.com/moby/swarmkit/v2/agent/exec"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	mobyplugin "github.com/moby/swarmkit/v2/node/plugin"
	"github.com/moby/swarmkit/v2/volumequeue"
)

const csiCallTimeout = 15 * time.Second

// volumeState keeps track of the state of a volume on this node.
type volumeState struct {
	// volume is the actual VolumeAssignment for this volume
	volume *api.VolumeAssignment
	// remove is true if the volume is to be removed, or false if it should be
	// active.
	remove bool
	// removeCallback is called when the volume is successfully removed.
	removeCallback func(id string)
}

// volumes is a map that keeps all the currently available volumes to the agent
// mapped by volume ID.
type volumes struct {
	// mu guards access to the volumes map.
	mu sync.RWMutex

	// volumes is a mapping of volume ID to volumeState
	volumes map[string]volumeState

	// plugins is the Manager, which provides translation to the CSI RPCs
	plugins plugin.Manager

	// pendingVolumes is a VolumeQueue which manages which volumes are
	// processed and when.
	pendingVolumes *volumequeue.VolumeQueue
}

// NewManager returns a place to store volumes.
func NewManager(pg mobyplugin.Getter, secrets exec.SecretGetter) exec.VolumesManager {
	r := &volumes{
		volumes:        map[string]volumeState{},
		plugins:        plugin.NewManager(pg, secrets),
		pendingVolumes: volumequeue.NewVolumeQueue(),
	}
	go r.retryVolumes()

	return r
}

// retryVolumes runs in a goroutine to retry failing volumes.
func (r *volumes) retryVolumes() {
	ctx := log.WithModule(context.Background(), "node/agent/csi")
	for {
		vid, attempt := r.pendingVolumes.Wait()

		dctx := log.WithFields(ctx, log.Fields{
			"volume.id": vid,
			"attempt":   fmt.Sprintf("%d", attempt),
		})

		// this case occurs when the Stop method has been called on
		// pendingVolumes, and means that we should pack up and exit.
		if vid == "" && attempt == 0 {
			break
		}
		r.tryVolume(dctx, vid, attempt)
	}
}

// tryVolume synchronously tries one volume. it puts the volume back into the
// queue if the attempt fails.
func (r *volumes) tryVolume(ctx context.Context, id string, attempt uint) {
	r.mu.RLock()
	vs, ok := r.volumes[id]
	r.mu.RUnlock()

	if !ok {
		return
	}

	// create a sub-context with a timeout. because we can only process one
	// volume at a time, if we rely on the server-side or default timeout, we
	// may be waiting a very long time for a particular volume to fail.
	//
	// TODO(dperny): there is almost certainly a more intelligent way to do
	// this. For example, we could:
	//
	//   * Change code such that we can service volumes managed by different
	//     plugins at the same time.
	//   * Take longer timeouts when we don't have any other volumes in the
	//     queue
	//   * Have interruptible attempts, so that if we're taking longer
	//     timeouts, we can abort them to service new volumes.
	//
	// These are too complicated to be worth the engineering effort at this
	// time.

	timeoutCtx, cancel := context.WithTimeout(ctx, csiCallTimeout)
	// always gotta call the WithTimeout cancel
	defer cancel()

	if !vs.remove {
		if err := r.publishVolume(timeoutCtx, vs.volume); err != nil {
			log.G(timeoutCtx).WithError(err).Info("publishing volume failed")
			r.pendingVolumes.Enqueue(id, attempt+1)
		}
	} else {
		if err := r.unpublishVolume(timeoutCtx, vs.volume); err != nil {
			log.G(timeoutCtx).WithError(err).Info("upublishing volume failed")
			r.pendingVolumes.Enqueue(id, attempt+1)
		} else {
			// if unpublishing was successful, then call the callback
			vs.removeCallback(id)
		}
	}
}

// Get returns a volume published path for the provided volume ID.  If the volume doesn't exist, returns empty string.
func (r *volumes) Get(volumeID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if vs, ok := r.volumes[volumeID]; ok {
		if vs.remove {
			// TODO(dperny): use a structured error
			return "", fmt.Errorf("volume being removed")
		}

		if p, err := r.plugins.Get(vs.volume.Driver.Name); err == nil {
			path := p.GetPublishedPath(volumeID)
			if path != "" {
				return path, nil
			}
			// don't put this line here, it spams like crazy.
			// log.L.WithField("method", "(*volumes).Get").Debugf("Path not published for volume:%v", volumeID)
		} else {
			return "", err
		}

	}
	return "", fmt.Errorf("%w: published path is unavailable", exec.ErrDependencyNotReady)
}

// Add adds one or more volumes to the volume map.
func (r *volumes) Add(volumes ...api.VolumeAssignment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, volume := range volumes {
		// if we get an Add operation, then we will always restart the retries.
		v := volume.Copy()
		r.volumes[volume.ID] = volumeState{
			volume: v,
		}
		// enqueue the volume so that we process it
		r.pendingVolumes.Enqueue(volume.ID, 0)
		log.L.WithField("method", "(*volumes).Add").Debugf("Add Volume: %v", volume.VolumeID)
	}
}

// Remove removes one or more volumes from this manager. callback is called
// whenever the removal is successful.
func (r *volumes) Remove(volumes []api.VolumeAssignment, callback func(id string)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, volume := range volumes {
		// if we get a Remove call, then we always restart the retries and
		// attempt removal.
		v := volume.Copy()
		r.volumes[volume.ID] = volumeState{
			volume:         v,
			remove:         true,
			removeCallback: callback,
		}
		r.pendingVolumes.Enqueue(volume.ID, 0)
	}
}

func (r *volumes) publishVolume(ctx context.Context, assignment *api.VolumeAssignment) error {
	log.G(ctx).Info("attempting to publish volume")
	p, err := r.plugins.Get(assignment.Driver.Name)
	if err != nil {
		return err
	}

	// even though this may have succeeded already, the call to NodeStageVolume
	// is idempotent, so we can retry it every time.
	if err := p.NodeStageVolume(ctx, assignment); err != nil {
		return err
	}

	log.G(ctx).Debug("staging volume succeeded, attempting to publish volume")

	return p.NodePublishVolume(ctx, assignment)
}

func (r *volumes) unpublishVolume(ctx context.Context, assignment *api.VolumeAssignment) error {
	log.G(ctx).Info("attempting to unpublish volume")
	p, err := r.plugins.Get(assignment.Driver.Name)
	if err != nil {
		return err
	}

	if err := p.NodeUnpublishVolume(ctx, assignment); err != nil {
		return err
	}

	return p.NodeUnstageVolume(ctx, assignment)
}

func (r *volumes) Plugins() exec.VolumePluginManager {
	return r.plugins
}

// taskRestrictedVolumesProvider restricts the ids to the task.
type taskRestrictedVolumesProvider struct {
	volumes   exec.VolumeGetter
	volumeIDs map[string]struct{}
}

func (sp *taskRestrictedVolumesProvider) Get(volumeID string) (string, error) {
	if _, ok := sp.volumeIDs[volumeID]; !ok {
		return "", fmt.Errorf("task not authorized to access volume %s", volumeID)
	}

	return sp.volumes.Get(volumeID)
}

// Restrict provides a getter that only allows access to the volumes
// referenced by the task.
func Restrict(volumes exec.VolumeGetter, t *api.Task) exec.VolumeGetter {
	vids := map[string]struct{}{}

	for _, v := range t.Volumes {
		vids[v.ID] = struct{}{}
	}

	return &taskRestrictedVolumesProvider{volumes: volumes, volumeIDs: vids}
}
