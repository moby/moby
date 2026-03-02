package libnetwork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
)

const (
	sandboxPrefix = "sandbox"
)

type epState struct {
	Eid string
	Nid string
}

type sbState struct {
	ID         string
	Cid        string
	c          *Controller
	dbIndex    uint64
	dbExists   bool
	Eps        []epState
	EpPriority map[string]int
	// external servers have to be persisted so that on restart of a live-restore
	// enabled daemon we get the external servers for the running containers.
	//
	// It is persisted as "ExtDNS2" for historical reasons. ExtDNS2 was used to
	// handle migration between docker < 1.14 and >= 1.14. Before version 1.14 we
	// used ExtDNS but with a []string. As it's unlikely that installations still
	// have state from before 1.14, we've dropped the migration code.
	ExtDNS []extDNSEntry `json:"ExtDNS2"`
}

func (sbs *sbState) Key() []string {
	return []string{sandboxPrefix, sbs.ID}
}

func (sbs *sbState) KeyPrefix() []string {
	return []string{sandboxPrefix}
}

func (sbs *sbState) Value() []byte {
	b, err := json.Marshal(sbs)
	if err != nil {
		return nil
	}
	return b
}

func (sbs *sbState) SetValue(value []byte) error {
	return json.Unmarshal(value, sbs)
}

func (sbs *sbState) Index() uint64 {
	sb, err := sbs.c.SandboxByID(sbs.ID)
	if err != nil {
		return sbs.dbIndex
	}

	maxIndex := max(sbs.dbIndex, sb.dbIndex)

	return maxIndex
}

func (sbs *sbState) SetIndex(index uint64) {
	sbs.dbIndex = index
	sbs.dbExists = true

	sb, err := sbs.c.SandboxByID(sbs.ID)
	if err != nil {
		return
	}

	sb.dbIndex = index
	sb.dbExists = true
}

func (sbs *sbState) Exists() bool {
	if sbs.dbExists {
		return sbs.dbExists
	}

	sb, err := sbs.c.SandboxByID(sbs.ID)
	if err != nil {
		return false
	}

	return sb.dbExists
}

func (sbs *sbState) Skip() bool {
	return false
}

func (sbs *sbState) New() datastore.KVObject {
	return &sbState{c: sbs.c}
}

func (sbs *sbState) CopyTo(o datastore.KVObject) error {
	dstSbs := o.(*sbState)
	dstSbs.c = sbs.c
	dstSbs.ID = sbs.ID
	dstSbs.Cid = sbs.Cid
	dstSbs.dbIndex = sbs.dbIndex
	dstSbs.dbExists = sbs.dbExists
	dstSbs.EpPriority = sbs.EpPriority

	dstSbs.Eps = append(dstSbs.Eps, sbs.Eps...)
	dstSbs.ExtDNS = append(dstSbs.ExtDNS, sbs.ExtDNS...)

	return nil
}

func (sb *Sandbox) storeUpdate(ctx context.Context) error {
	sbs := &sbState{
		c:          sb.controller,
		ID:         sb.id,
		Cid:        sb.containerID,
		EpPriority: sb.epPriority,
		ExtDNS:     sb.extDNS,
	}

retry:
	sbs.Eps = nil
	for _, ep := range sb.Endpoints() {
		// If the endpoint is not persisted then do not add it to
		// the sandbox checkpoint
		if ep.Skip() {
			continue
		}

		sbs.Eps = append(sbs.Eps, epState{
			Nid: ep.getNetwork().ID(),
			Eid: ep.ID(),
		})
	}

	err := sb.controller.updateToStore(ctx, sbs)
	if errors.Is(err, datastore.ErrKeyModified) {
		// When we get ErrKeyModified it is sufficient to just
		// go back and retry.  No need to get the object from
		// the store because we always regenerate the store
		// state from in memory sandbox state
		goto retry
	}

	return err
}

func (sb *Sandbox) storeDelete() error {
	return sb.controller.store.DeleteObject(&sbState{
		c:        sb.controller,
		ID:       sb.id,
		Cid:      sb.containerID,
		dbExists: sb.dbExists,
	})
}

// sandboxRestore restores Sandbox objects from the store, deleting them if they're not active.
func (c *Controller) sandboxRestore(activeSandboxes map[string]any) error {
	sandboxStates, err := c.store.List(&sbState{c: c})
	if err != nil {
		if errors.Is(err, datastore.ErrKeyNotFound) {
			// It's normal for no sandboxes to be found. Just bail out.
			return nil
		}
		return fmt.Errorf("failed to get sandboxes: %v", err)
	}

	for _, s := range sandboxStates {
		sbs := s.(*sbState)
		sb := &Sandbox{
			id:                 sbs.ID,
			controller:         sbs.c,
			containerID:        sbs.Cid,
			epPriority:         sbs.EpPriority,
			extDNS:             sbs.ExtDNS,
			endpoints:          []*Endpoint{},
			populatedEndpoints: map[string]struct{}{},
			dbIndex:            sbs.dbIndex,
			isStub:             true,
			dbExists:           true,
		}

		create := true
		isRestore := false
		if val, ok := activeSandboxes[sb.ID()]; ok {
			sb.isStub = false
			isRestore = true
			opts := val.([]SandboxOption)
			sb.processOptions(opts...)
			sb.restoreHostsPath()
			sb.restoreResolvConfPath()
			create = !sb.config.useDefaultSandBox
		}

		ctx := context.TODO()
		ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
			"sid":       stringid.TruncateID(sb.ID()),
			"cid":       stringid.TruncateID(sb.ContainerID()),
			"isRestore": isRestore,
		}))

		sb.osSbox, err = osl.NewSandbox(sb.Key(), create, isRestore)
		if err != nil {
			log.G(ctx).WithError(err).Error("Failed to create osl sandbox while trying to restore sandbox")
			continue
		}

		c.mu.Lock()
		c.sandboxes[sb.id] = sb
		c.mu.Unlock()

		for _, eps := range sbs.Eps {
			ctx := log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
				"nid": stringid.TruncateID(eps.Nid),
				"eid": stringid.TruncateID(eps.Eid),
			}))
			// If the Network or Endpoint can't be loaded from the store, log and continue. Something
			// might go wrong later, but it might just be a reference to a deleted network/endpoint
			// (in which case, the best thing to do is to continue to run/delete the Sandbox with the
			// available configuration).
			n, err := c.getNetworkFromStore(eps.Nid)
			if err != nil {
				log.G(ctx).WithError(err).Warn("Failed to restore endpoint, getNetworkFromStore failed")
				continue
			}
			ep, err := n.getEndpointFromStore(eps.Eid)
			if err != nil {
				log.G(ctx).WithError(err).Warn("Failed to restore endpoint, getEndpointFromStore failed")
				continue
			}
			sb.addEndpoint(ep)
		}

		if !isRestore {
			log.G(ctx).Info("Removing stale sandbox")
			if err := sb.delete(context.WithoutCancel(ctx), true); err != nil {
				log.G(ctx).WithError(err).Warn("Failed to delete sandbox while trying to clean up")
			}
			continue
		}

		for _, ep := range sb.endpoints {
			sb.populatedEndpoints[ep.id] = struct{}{}
		}

		// reconstruct osl sandbox field
		if !sb.config.useDefaultSandBox {
			if err := sb.restoreOslSandbox(); err != nil {
				log.G(ctx).WithError(err).Error("Failed to populate fields for osl sandbox")
				continue
			}
		} else {
			// FIXME(thaJeztah): osSbox (and thus defOsSbox) is always nil on non-Linux: move this code to Linux-only files.
			c.defOsSboxOnce.Do(func() {
				c.defOsSbox = sb.osSbox
			})
		}

		for _, ep := range sb.endpoints {
			if !c.isAgent() {
				n := ep.getNetwork()
				if !c.isSwarmNode() || n.Scope() != scope.Swarm || !n.driverIsMultihost() {
					n.updateSvcRecord(context.WithoutCancel(ctx), ep, true)
				}
			}
		}
	}

	return nil
}
