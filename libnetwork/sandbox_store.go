package libnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/scope"
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

	maxIndex := sb.dbIndex
	if sbs.dbIndex > maxIndex {
		maxIndex = sbs.dbIndex
	}

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

func (sb *Sandbox) storeUpdate() error {
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

	err := sb.controller.updateToStore(sbs)
	if err == datastore.ErrKeyModified {
		// When we get ErrKeyModified it is sufficient to just
		// go back and retry.  No need to get the object from
		// the store because we always regenerate the store
		// state from in memory sandbox state
		goto retry
	}

	return err
}

func (sb *Sandbox) storeDelete() error {
	return sb.controller.deleteFromStore(&sbState{
		c:        sb.controller,
		ID:       sb.id,
		Cid:      sb.containerID,
		dbIndex:  sb.dbIndex,
		dbExists: sb.dbExists,
	})
}

func (c *Controller) sandboxCleanup(activeSandboxes map[string]interface{}) error {
	store := c.getStore()
	if store == nil {
		return fmt.Errorf("could not find local scope store")
	}

	sandboxStates, err := store.List(&sbState{c: c})
	if err != nil {
		if err == datastore.ErrKeyNotFound {
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
			extDNS:             sbs.ExtDNS,
			endpoints:          []*Endpoint{},
			populatedEndpoints: map[string]struct{}{},
			dbIndex:            sbs.dbIndex,
			isStub:             true,
			dbExists:           true,
		}

		msg := " for cleanup"
		create := true
		isRestore := false
		if val, ok := activeSandboxes[sb.ID()]; ok {
			msg = ""
			sb.isStub = false
			isRestore = true
			opts := val.([]SandboxOption)
			sb.processOptions(opts...)
			sb.restorePath()
			create = !sb.config.useDefaultSandBox
		}
		sb.osSbox, err = osl.NewSandbox(sb.Key(), create, isRestore)
		if err != nil {
			log.G(context.TODO()).Errorf("failed to create osl sandbox while trying to restore sandbox %.7s%s: %v", sb.ID(), msg, err)
			continue
		}

		c.mu.Lock()
		c.sandboxes[sb.id] = sb
		c.mu.Unlock()

		for _, eps := range sbs.Eps {
			n, err := c.getNetworkFromStore(eps.Nid)
			var ep *Endpoint
			if err != nil {
				log.G(context.TODO()).Errorf("getNetworkFromStore for nid %s failed while trying to build sandbox for cleanup: %v", eps.Nid, err)
				ep = &Endpoint{
					id: eps.Eid,
					network: &Network{
						id:      eps.Nid,
						ctrlr:   c,
						drvOnce: &sync.Once{},
						persist: true,
					},
					sandboxID: sbs.ID,
				}
			} else {
				ep, err = n.getEndpointFromStore(eps.Eid)
				if err != nil {
					log.G(context.TODO()).Errorf("getEndpointFromStore for eid %s failed while trying to build sandbox for cleanup: %v", eps.Eid, err)
					ep = &Endpoint{
						id:        eps.Eid,
						network:   n,
						sandboxID: sbs.ID,
					}
				}
			}
			if _, ok := activeSandboxes[sb.ID()]; ok && err != nil {
				log.G(context.TODO()).Errorf("failed to restore endpoint %s in %s for container %s due to %v", eps.Eid, eps.Nid, sb.ContainerID(), err)
				continue
			}
			sb.addEndpoint(ep)
		}

		if _, ok := activeSandboxes[sb.ID()]; !ok {
			log.G(context.TODO()).Infof("Removing stale sandbox %s (%s)", sb.id, sb.containerID)
			if err := sb.delete(true); err != nil {
				log.G(context.TODO()).Errorf("Failed to delete sandbox %s while trying to cleanup: %v", sb.id, err)
			}
			continue
		}

		// reconstruct osl sandbox field
		if !sb.config.useDefaultSandBox {
			if err := sb.restoreOslSandbox(); err != nil {
				log.G(context.TODO()).Errorf("failed to populate fields for osl sandbox %s: %v", sb.ID(), err)
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
					n.updateSvcRecord(ep, true)
				}
			}
		}
	}

	return nil
}
