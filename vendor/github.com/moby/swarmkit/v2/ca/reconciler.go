package ca

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/equality"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/pkg/errors"
)

// IssuanceStateRotateMaxBatchSize is the maximum number of nodes we'll tell to rotate their certificates in any given update
const IssuanceStateRotateMaxBatchSize = 30

func hasIssuer(n *api.Node, info *IssuerInfo) bool {
	if n.Description == nil || n.Description.TLSInfo == nil {
		return false
	}
	return bytes.Equal(info.Subject, n.Description.TLSInfo.CertIssuerSubject) && bytes.Equal(info.PublicKey, n.Description.TLSInfo.CertIssuerPublicKey)
}

var errRootRotationChanged = errors.New("target root rotation has changed")

// rootRotationReconciler keeps track of all the nodes in the store so that we can determine which ones need reconciliation when nodes are updated
// or the root CA is updated.  This is meant to be used with watches on nodes and the cluster, and provides functions to be called when the
// cluster's RootCA has changed and when a node is added, updated, or removed.
type rootRotationReconciler struct {
	mu                  sync.Mutex
	clusterID           string
	batchUpdateInterval time.Duration
	ctx                 context.Context
	store               *store.MemoryStore

	currentRootCA    *api.RootCA
	currentIssuer    IssuerInfo
	unconvergedNodes map[string]*api.Node

	wg     sync.WaitGroup
	cancel func()
}

// IssuerFromAPIRootCA returns the desired issuer given an API root CA object
func IssuerFromAPIRootCA(rootCA *api.RootCA) (*IssuerInfo, error) {
	wantedIssuer := rootCA.CACert
	if rootCA.RootRotation != nil {
		wantedIssuer = rootCA.RootRotation.CACert
	}
	issuerCerts, err := helpers.ParseCertificatesPEM(wantedIssuer)
	if err != nil {
		return nil, errors.Wrap(err, "invalid certificate in cluster root CA object")
	}
	if len(issuerCerts) == 0 {
		return nil, errors.New("invalid certificate in cluster root CA object")
	}
	return &IssuerInfo{
		Subject:   issuerCerts[0].RawSubject,
		PublicKey: issuerCerts[0].RawSubjectPublicKeyInfo,
	}, nil
}

// assumption:  UpdateRootCA will never be called with a `nil` root CA because the caller will be acting in response to
// a store update event
func (r *rootRotationReconciler) UpdateRootCA(newRootCA *api.RootCA) {
	issuerInfo, err := IssuerFromAPIRootCA(newRootCA)
	if err != nil {
		log.G(r.ctx).WithError(err).Error("unable to update process the current root CA")
		return
	}

	var (
		shouldStartNewLoop, waitForPrevLoop bool
		loopCtx                             context.Context
	)
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		if shouldStartNewLoop {
			if waitForPrevLoop {
				r.wg.Wait()
			}
			r.wg.Add(1)
			go r.runReconcilerLoop(loopCtx, newRootCA)
		}
	}()

	// check if the issuer has changed, first
	if reflect.DeepEqual(&r.currentIssuer, issuerInfo) {
		r.currentRootCA = newRootCA
		return
	}
	// If the issuer has changed, iterate through all the nodes to figure out which ones need rotation
	if newRootCA.RootRotation != nil {
		var nodes []*api.Node
		r.store.View(func(tx store.ReadTx) {
			nodes, err = store.FindNodes(tx, store.All)
		})
		if err != nil {
			log.G(r.ctx).WithError(err).Error("unable to list nodes, so unable to process the current root CA")
			return
		}

		// from here on out, there will be no more errors that cause us to have to abandon updating the Root CA,
		// so we can start making changes to r's fields
		r.unconvergedNodes = make(map[string]*api.Node)
		for _, n := range nodes {
			if !hasIssuer(n, issuerInfo) {
				r.unconvergedNodes[n.ID] = n
			}
		}
		shouldStartNewLoop = true
		if r.cancel != nil { // there's already a loop going, so cancel it
			r.cancel()
			waitForPrevLoop = true
		}
		loopCtx, r.cancel = context.WithCancel(r.ctx)
	} else {
		r.unconvergedNodes = nil
	}
	r.currentRootCA = newRootCA
	r.currentIssuer = *issuerInfo
}

// assumption:  UpdateNode will never be called with a `nil` node because the caller will be acting in response to
// a store update event
func (r *rootRotationReconciler) UpdateNode(node *api.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// if we're not in the middle of a root rotation ignore the update
	if r.currentRootCA == nil || r.currentRootCA.RootRotation == nil {
		return
	}
	if hasIssuer(node, &r.currentIssuer) {
		delete(r.unconvergedNodes, node.ID)
	} else {
		r.unconvergedNodes[node.ID] = node
	}
}

// assumption:  DeleteNode will never be called with a `nil` node because the caller will be acting in response to
// a store update event
func (r *rootRotationReconciler) DeleteNode(node *api.Node) {
	r.mu.Lock()
	delete(r.unconvergedNodes, node.ID)
	r.mu.Unlock()
}

func (r *rootRotationReconciler) runReconcilerLoop(ctx context.Context, loopRootCA *api.RootCA) {
	defer r.wg.Done()
	for {
		r.mu.Lock()
		if len(r.unconvergedNodes) == 0 {
			r.mu.Unlock()

			err := r.store.Update(func(tx store.Tx) error {
				return r.finishRootRotation(tx, loopRootCA)
			})
			if err == nil {
				log.G(r.ctx).Info("completed root rotation")
				return
			}
			log.G(r.ctx).WithError(err).Error("could not complete root rotation")
			if err == errRootRotationChanged {
				// if the root rotation has changed, this loop will be cancelled anyway, so may as well abort early
				return
			}
		} else {
			var toUpdate []*api.Node
			for _, n := range r.unconvergedNodes {
				iState := n.Certificate.Status.State
				if iState != api.IssuanceStateRenew && iState != api.IssuanceStatePending && iState != api.IssuanceStateRotate {
					n = n.Copy()
					n.Certificate.Status.State = api.IssuanceStateRotate
					toUpdate = append(toUpdate, n)
					if len(toUpdate) >= IssuanceStateRotateMaxBatchSize {
						break
					}
				}
			}
			r.mu.Unlock()

			if err := r.batchUpdateNodes(toUpdate); err != nil {
				log.G(r.ctx).WithError(err).Errorf("store error when trying to batch update %d nodes to request certificate rotation", len(toUpdate))
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(r.batchUpdateInterval):
		}
	}
}

// This function assumes that the expected root CA has root rotation.  This is intended to be used by
// `reconcileNodeRootsAndCerts`, which uses the root CA from the `lastSeenClusterRootCA`, and checks
// that it has a root rotation before calling this function.
func (r *rootRotationReconciler) finishRootRotation(tx store.Tx, expectedRootCA *api.RootCA) error {
	cluster := store.GetCluster(tx, r.clusterID)
	if cluster == nil {
		return fmt.Errorf("unable to get cluster %s", r.clusterID)
	}

	// If the RootCA object has changed (because another root rotation was started or because some other node
	// had finished the root rotation), we cannot finish the root rotation that we were working on.
	if !equality.RootCAEqualStable(expectedRootCA, &cluster.RootCA) {
		return errRootRotationChanged
	}

	var signerCert []byte
	if len(cluster.RootCA.RootRotation.CAKey) > 0 {
		signerCert = cluster.RootCA.RootRotation.CACert
	}
	// we don't actually have to parse out the default node expiration from the cluster - we are just using
	// the ca.RootCA object to generate new tokens and the digest
	updatedRootCA, err := NewRootCA(cluster.RootCA.RootRotation.CACert, signerCert, cluster.RootCA.RootRotation.CAKey,
		DefaultNodeCertExpiration, nil)
	if err != nil {
		return errors.Wrap(err, "invalid cluster root rotation object")
	}
	cluster.RootCA = api.RootCA{
		CACert:     cluster.RootCA.RootRotation.CACert,
		CAKey:      cluster.RootCA.RootRotation.CAKey,
		CACertHash: updatedRootCA.Digest.String(),
		JoinTokens: api.JoinTokens{
			Worker:  GenerateJoinToken(&updatedRootCA, cluster.FIPS),
			Manager: GenerateJoinToken(&updatedRootCA, cluster.FIPS),
		},
		LastForcedRotation: cluster.RootCA.LastForcedRotation,
	}
	return store.UpdateCluster(tx, cluster)
}

func (r *rootRotationReconciler) batchUpdateNodes(toUpdate []*api.Node) error {
	if len(toUpdate) == 0 {
		return nil
	}
	err := r.store.Batch(func(batch *store.Batch) error {
		// Directly update the nodes rather than get + update, and ignore version errors.  Since
		// `rootRotationReconciler` should be hooked up to all node update/delete/create events, we should have
		// close to the latest versions of all the nodes.  If not, the node will updated later and the
		// next batch of updates should catch it.
		for _, n := range toUpdate {
			if err := batch.Update(func(tx store.Tx) error {
				return store.UpdateNode(tx, n)
			}); err != nil && err != store.ErrSequenceConflict {
				log.G(r.ctx).WithError(err).Errorf("unable to update node %s to request a certificate rotation", n.ID)
			}
		}
		return nil
	})
	return err
}
