/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metadata

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/gc"
	"github.com/containerd/containerd/log"
	bolt "go.etcd.io/bbolt"
)

const (
	// ResourceUnknown specifies an unknown resource
	ResourceUnknown gc.ResourceType = iota
	// ResourceContent specifies a content resource
	ResourceContent
	// ResourceSnapshot specifies a snapshot resource
	ResourceSnapshot
	// ResourceContainer specifies a container resource
	ResourceContainer
	// ResourceTask specifies a task resource
	ResourceTask
	// ResourceLease specifies a lease
	ResourceLease
	// ResourceIngest specifies a content ingest
	ResourceIngest
	// resourceEnd is the end of specified resource types
	resourceEnd
	// ResourceStream specifies a stream
	ResourceStream
)

const (
	resourceContentFlat  = ResourceContent | 0x20
	resourceSnapshotFlat = ResourceSnapshot | 0x20
)

var (
	labelGCRoot       = []byte("containerd.io/gc.root")
	labelGCRef        = []byte("containerd.io/gc.ref.")
	labelGCSnapRef    = []byte("containerd.io/gc.ref.snapshot.")
	labelGCContentRef = []byte("containerd.io/gc.ref.content")
	labelGCExpire     = []byte("containerd.io/gc.expire")
	labelGCFlat       = []byte("containerd.io/gc.flat")
)

// CollectionContext manages a resource collection during a single run of
// the garbage collector. The context is responsible for managing access to
// resources as well as tracking removal.
// Implementations should defer any longer running operations to the Finish
// function and optimize other functions for running fast during garbage
// collection write locks.
type CollectionContext interface {
	// Sends all known resources
	All(func(gc.Node))

	// Active sends all active resources
	// Leased resources may be excluded since lease ownership should take
	// precedence over active status.
	Active(namespace string, fn func(gc.Node))

	// Leased sends all resources associated with the given lease
	Leased(namespace, lease string, fn func(gc.Node))

	// Remove marks the given resource as removed
	Remove(gc.Node)

	// Cancel is called to cleanup a context after a failed collection
	Cancel() error

	// Finish is called to cleanup a context after a successful collection
	Finish() error
}

// Collector is an interface to manage resource collection for any collectible
// resource registered for garbage collection.
type Collector interface {
	StartCollection(context.Context) (CollectionContext, error)

	ReferenceLabel() string
}

type gcContext struct {
	labelHandlers []referenceLabelHandler
	contexts      map[gc.ResourceType]CollectionContext
}

type referenceLabelHandler struct {
	key []byte
	fn  func(string, []byte, []byte, func(gc.Node))
}

func startGCContext(ctx context.Context, collectors map[gc.ResourceType]Collector) *gcContext {
	var contexts map[gc.ResourceType]CollectionContext
	labelHandlers := []referenceLabelHandler{
		{
			key: labelGCContentRef,
			fn: func(ns string, k, v []byte, fn func(gc.Node)) {
				if ks := string(k); ks != string(labelGCContentRef) {
					// Allow reference naming separated by . or /, ignore names
					if ks[len(labelGCContentRef)] != '.' && ks[len(labelGCContentRef)] != '/' {
						return
					}
				}

				fn(gcnode(ResourceContent, ns, string(v)))
			},
		},
		{
			key: labelGCSnapRef,
			fn: func(ns string, k, v []byte, fn func(gc.Node)) {
				snapshotter := k[len(labelGCSnapRef):]
				if i := bytes.IndexByte(snapshotter, '/'); i >= 0 {
					snapshotter = snapshotter[:i]
				}
				fn(gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", snapshotter, v)))
			},
		},
	}
	if len(collectors) > 0 {
		contexts = map[gc.ResourceType]CollectionContext{}
		for rt, collector := range collectors {
			rt := rt
			c, err := collector.StartCollection(ctx)
			if err != nil {
				// Only skipping this resource this round
				continue
			}

			if reflabel := collector.ReferenceLabel(); reflabel != "" {
				key := append(labelGCRef, reflabel...)
				labelHandlers = append(labelHandlers, referenceLabelHandler{
					key: key,
					fn: func(ns string, k, v []byte, fn func(gc.Node)) {
						if ks := string(k); ks != string(key) {
							// Allow reference naming separated by . or /, ignore names
							if ks[len(key)] != '.' && ks[len(key)] != '/' {
								return
							}
						}

						fn(gcnode(rt, ns, string(v)))
					},
				})
			}
			contexts[rt] = c
		}
		// Sort labelHandlers to ensure key seeking is always forwardS
		sort.Slice(labelHandlers, func(i, j int) bool {
			return bytes.Compare(labelHandlers[i].key, labelHandlers[j].key) < 0
		})
	}
	return &gcContext{
		labelHandlers: labelHandlers,
		contexts:      contexts,
	}
}

func (c *gcContext) all(fn func(gc.Node)) {
	for _, gctx := range c.contexts {
		gctx.All(fn)
	}
}

func (c *gcContext) active(namespace string, fn func(gc.Node)) {
	for _, gctx := range c.contexts {
		gctx.Active(namespace, fn)
	}
}

func (c *gcContext) leased(namespace, lease string, fn func(gc.Node)) {
	for _, gctx := range c.contexts {
		gctx.Leased(namespace, lease, fn)
	}
}

func (c *gcContext) cancel(ctx context.Context) {
	for _, gctx := range c.contexts {
		if err := gctx.Cancel(); err != nil {
			log.G(ctx).WithError(err).Error("failed to cancel collection context")
		}
	}
}

func (c *gcContext) finish(ctx context.Context) {
	for _, gctx := range c.contexts {
		if err := gctx.Finish(); err != nil {
			log.G(ctx).WithError(err).Error("failed to finish collection context")
		}
	}
}

// scanRoots sends the given channel "root" resources that are certainly used.
// The caller could look the references of the resources to find all resources that are used.
func (c *gcContext) scanRoots(ctx context.Context, tx *bolt.Tx, nc chan<- gc.Node) error {
	v1bkt := tx.Bucket(bucketKeyVersion)
	if v1bkt == nil {
		return nil
	}

	expThreshold := time.Now()

	// iterate through each namespace
	v1c := v1bkt.Cursor()

	// cerr indicates the scan did not successfully send all
	// the roots. The scan does not need to be cancelled but
	// must return error at the end.
	var cerr error
	fn := func(n gc.Node) {
		select {
		case nc <- n:
		case <-ctx.Done():
			cerr = ctx.Err()
		}
	}

	for k, v := v1c.First(); k != nil; k, v = v1c.Next() {
		if v != nil {
			continue
		}
		nbkt := v1bkt.Bucket(k)
		ns := string(k)

		lbkt := nbkt.Bucket(bucketKeyObjectLeases)
		if lbkt != nil {
			if err := lbkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}
				libkt := lbkt.Bucket(k)
				var flat bool

				if lblbkt := libkt.Bucket(bucketKeyObjectLabels); lblbkt != nil {
					if expV := lblbkt.Get(labelGCExpire); expV != nil {
						exp, err := time.Parse(time.RFC3339, string(expV))
						if err != nil {
							// label not used, log and continue to use lease
							log.G(ctx).WithError(err).WithField("lease", string(k)).Infof("ignoring invalid expiration value %q", string(expV))
						} else if expThreshold.After(exp) {
							// lease has expired, skip
							return nil
						}
					}

					if flatV := lblbkt.Get(labelGCFlat); flatV != nil {
						flat = true
					}
				}

				fn(gcnode(ResourceLease, ns, string(k)))

				// Emit content and snapshots as roots instead of implementing
				// in references. Since leases cannot be referenced there is
				// no need to allow the lookup to be recursive, handling here
				// therefore reduces the number of database seeks.

				ctype := ResourceContent
				if flat {
					ctype = resourceContentFlat
				}

				cbkt := libkt.Bucket(bucketKeyObjectContent)
				if cbkt != nil {
					if err := cbkt.ForEach(func(k, v []byte) error {
						fn(gcnode(ctype, ns, string(k)))
						return nil
					}); err != nil {
						return err
					}
				}

				stype := ResourceSnapshot
				if flat {
					stype = resourceSnapshotFlat
				}

				sbkt := libkt.Bucket(bucketKeyObjectSnapshots)
				if sbkt != nil {
					if err := sbkt.ForEach(func(sk, sv []byte) error {
						if sv != nil {
							return nil
						}
						snbkt := sbkt.Bucket(sk)

						return snbkt.ForEach(func(k, v []byte) error {
							fn(gcnode(stype, ns, fmt.Sprintf("%s/%s", sk, k)))
							return nil
						})
					}); err != nil {
						return err
					}
				}

				ibkt := libkt.Bucket(bucketKeyObjectIngests)
				if ibkt != nil {
					if err := ibkt.ForEach(func(k, v []byte) error {
						fn(gcnode(ResourceIngest, ns, string(k)))
						return nil
					}); err != nil {
						return err
					}
				}

				c.leased(ns, string(k), fn)

				return nil
			}); err != nil {
				return err
			}
		}

		ibkt := nbkt.Bucket(bucketKeyObjectImages)
		if ibkt != nil {
			if err := ibkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}

				target := ibkt.Bucket(k).Bucket(bucketKeyTarget)
				if target != nil {
					contentKey := string(target.Get(bucketKeyDigest))
					fn(gcnode(ResourceContent, ns, contentKey))
				}
				return c.sendLabelRefs(ns, ibkt.Bucket(k), fn)
			}); err != nil {
				return err
			}
		}

		cbkt := nbkt.Bucket(bucketKeyObjectContent)
		if cbkt != nil {
			ibkt := cbkt.Bucket(bucketKeyObjectIngests)
			if ibkt != nil {
				if err := ibkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					ea, err := readExpireAt(ibkt.Bucket(k))
					if err != nil {
						return err
					}
					if ea == nil || expThreshold.After(*ea) {
						return nil
					}
					fn(gcnode(ResourceIngest, ns, string(k)))
					return nil
				}); err != nil {
					return err
				}
			}
			cbkt = cbkt.Bucket(bucketKeyObjectBlob)
			if cbkt != nil {
				if err := cbkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}

					if isRootRef(cbkt.Bucket(k)) {
						fn(gcnode(ResourceContent, ns, string(k)))
					}

					return nil
				}); err != nil {
					return err
				}
			}
		}

		cbkt = nbkt.Bucket(bucketKeyObjectContainers)
		if cbkt != nil {
			if err := cbkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}

				cibkt := cbkt.Bucket(k)
				snapshotter := string(cibkt.Get(bucketKeySnapshotter))
				if snapshotter != "" {
					ss := string(cibkt.Get(bucketKeySnapshotKey))
					fn(gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", snapshotter, ss)))
				}

				return c.sendLabelRefs(ns, cibkt, fn)
			}); err != nil {
				return err
			}
		}

		sbkt := nbkt.Bucket(bucketKeyObjectSnapshots)
		if sbkt != nil {
			if err := sbkt.ForEach(func(sk, sv []byte) error {
				if sv != nil {
					return nil
				}
				snbkt := sbkt.Bucket(sk)

				return snbkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					if isRootRef(snbkt.Bucket(k)) {
						fn(gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", sk, k)))
					}
					return nil
				})
			}); err != nil {
				return err
			}
		}

		bbkt := nbkt.Bucket(bucketKeyObjectSandboxes)
		if bbkt != nil {
			if err := bbkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}

				sbbkt := bbkt.Bucket(k)
				return c.sendLabelRefs(ns, sbbkt, fn)
			}); err != nil {
				return err
			}
		}

		c.active(ns, fn)
	}
	return cerr
}

// references finds the resources that are reachable from the given node.
func (c *gcContext) references(ctx context.Context, tx *bolt.Tx, node gc.Node, fn func(gc.Node)) error {
	switch node.Type {
	case ResourceContent:
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectContent, bucketKeyObjectBlob, []byte(node.Key))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}

		return c.sendLabelRefs(node.Namespace, bkt, fn)
	case ResourceSnapshot, resourceSnapshotFlat:
		ss, name, ok := strings.Cut(node.Key, "/")
		if !ok {
			return fmt.Errorf("invalid snapshot gc key %s", node.Key)
		}
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectSnapshots, []byte(ss), []byte(name))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}

		if pv := bkt.Get(bucketKeyParent); len(pv) > 0 {
			fn(gcnode(node.Type, node.Namespace, fmt.Sprintf("%s/%s", ss, pv)))
		}

		// Do not send labeled references for flat snapshot refs
		if node.Type == resourceSnapshotFlat {
			return nil
		}

		return c.sendLabelRefs(node.Namespace, bkt, fn)
	case ResourceIngest:
		// Send expected value
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectContent, bucketKeyObjectIngests, []byte(node.Key))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}
		// Load expected
		expected := bkt.Get(bucketKeyExpected)
		if len(expected) > 0 {
			fn(gcnode(ResourceContent, node.Namespace, string(expected)))
		}
		return nil
	}

	return nil
}

// scanAll finds all resources regardless whether the resources are used or not.
func (c *gcContext) scanAll(ctx context.Context, tx *bolt.Tx, fn func(ctx context.Context, n gc.Node) error) error {
	v1bkt := tx.Bucket(bucketKeyVersion)
	if v1bkt == nil {
		return nil
	}

	// iterate through each namespace
	v1c := v1bkt.Cursor()

	for k, v := v1c.First(); k != nil; k, v = v1c.Next() {
		if v != nil {
			continue
		}
		nbkt := v1bkt.Bucket(k)
		ns := string(k)

		lbkt := nbkt.Bucket(bucketKeyObjectLeases)
		if lbkt != nil {
			if err := lbkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}
				return fn(ctx, gcnode(ResourceLease, ns, string(k)))
			}); err != nil {
				return err
			}
		}

		sbkt := nbkt.Bucket(bucketKeyObjectSnapshots)
		if sbkt != nil {
			if err := sbkt.ForEach(func(sk, sv []byte) error {
				if sv != nil {
					return nil
				}
				snbkt := sbkt.Bucket(sk)
				return snbkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					node := gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", sk, k))
					return fn(ctx, node)
				})
			}); err != nil {
				return err
			}
		}

		cbkt := nbkt.Bucket(bucketKeyObjectContent)
		if cbkt != nil {
			ibkt := cbkt.Bucket(bucketKeyObjectIngests)
			if ibkt != nil {
				if err := ibkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					node := gcnode(ResourceIngest, ns, string(k))
					return fn(ctx, node)
				}); err != nil {
					return err
				}
			}

			cbkt = cbkt.Bucket(bucketKeyObjectBlob)
			if cbkt != nil {
				if err := cbkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					node := gcnode(ResourceContent, ns, string(k))
					return fn(ctx, node)
				}); err != nil {
					return err
				}
			}
		}
	}

	c.all(func(n gc.Node) {
		_ = fn(ctx, n)
	})

	return nil
}

// remove all buckets for the given node.
func (c *gcContext) remove(ctx context.Context, tx *bolt.Tx, node gc.Node) (interface{}, error) {
	v1bkt := tx.Bucket(bucketKeyVersion)
	if v1bkt == nil {
		return nil, nil
	}

	nsbkt := v1bkt.Bucket([]byte(node.Namespace))
	if nsbkt == nil {
		// Still remove object if refenced outside the db
		if cc, ok := c.contexts[node.Type]; ok {
			cc.Remove(node)
		}
		return nil, nil
	}

	switch node.Type {
	case ResourceContent:
		cbkt := nsbkt.Bucket(bucketKeyObjectContent)
		if cbkt != nil {
			cbkt = cbkt.Bucket(bucketKeyObjectBlob)
		}
		if cbkt != nil {
			log.G(ctx).WithField("key", node.Key).Debug("remove content")
			return nil, cbkt.DeleteBucket([]byte(node.Key))
		}
	case ResourceSnapshot:
		sbkt := nsbkt.Bucket(bucketKeyObjectSnapshots)
		if sbkt != nil {
			ss, key, ok := strings.Cut(node.Key, "/")
			if !ok {
				return nil, fmt.Errorf("invalid snapshot gc key %s", node.Key)
			}
			ssbkt := sbkt.Bucket([]byte(ss))
			if ssbkt != nil {
				log.G(ctx).WithField("key", key).WithField("snapshotter", ss).Debug("remove snapshot")
				return &eventstypes.SnapshotRemove{
					Key:         key,
					Snapshotter: ss,
				}, ssbkt.DeleteBucket([]byte(key))
			}
		}
	case ResourceLease:
		lbkt := nsbkt.Bucket(bucketKeyObjectLeases)
		if lbkt != nil {
			return nil, lbkt.DeleteBucket([]byte(node.Key))
		}
	case ResourceIngest:
		ibkt := nsbkt.Bucket(bucketKeyObjectContent)
		if ibkt != nil {
			ibkt = ibkt.Bucket(bucketKeyObjectIngests)
		}
		if ibkt != nil {
			log.G(ctx).WithField("ref", node.Key).Debug("remove ingest")
			return nil, ibkt.DeleteBucket([]byte(node.Key))
		}
	default:
		cc, ok := c.contexts[node.Type]
		if ok {
			cc.Remove(node)
		} else {
			log.G(ctx).WithField("ref", node.Key).WithField("type", node.Type).Info("no remove defined for resource")
		}
	}

	return nil, nil
}

// sendLabelRefs sends all snapshot and content references referred to by the labels in the bkt
func (c *gcContext) sendLabelRefs(ns string, bkt *bolt.Bucket, fn func(gc.Node)) error {
	lbkt := bkt.Bucket(bucketKeyObjectLabels)
	if lbkt != nil {
		lc := lbkt.Cursor()
		for i := range c.labelHandlers {
			labelRef := string(c.labelHandlers[i].key)
			for k, v := lc.Seek(c.labelHandlers[i].key); k != nil && strings.HasPrefix(string(k), labelRef); k, v = lc.Next() {
				c.labelHandlers[i].fn(ns, k, v, fn)
			}
		}
	}
	return nil
}

func isRootRef(bkt *bolt.Bucket) bool {
	lbkt := bkt.Bucket(bucketKeyObjectLabels)
	if lbkt != nil {
		rv := lbkt.Get(labelGCRoot)
		if rv != nil {
			// TODO: interpret rv as a timestamp and skip if expired
			return true
		}
	}
	return false
}

func gcnode(t gc.ResourceType, ns, key string) gc.Node {
	return gc.Node{
		Type:      t,
		Namespace: ns,
		Key:       key,
	}
}
