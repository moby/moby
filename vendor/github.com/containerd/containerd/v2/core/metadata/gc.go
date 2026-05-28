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
	"sync"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/log"
	bolt "go.etcd.io/bbolt"

	"github.com/containerd/containerd/v2/pkg/gc"
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
	// ResourceImage specifies an image
	ResourceImage
	// ResourceLease specifies a lease
	ResourceLease
	// ResourceIngest specifies a content ingest
	ResourceIngest
	// resourceEnd is the end of specified resource types
	resourceEnd
	// ResourceStream specifies a stream
	ResourceStream
	// ResourceMount specifies a mount
	ResourceMount
)

const (
	resourceContentFlat  = ResourceContent | 0x20
	resourceSnapshotFlat = ResourceSnapshot | 0x20
	resourceImageFlat    = ResourceImage | 0x20
)

var (
	labelGCRoot = []byte("containerd.io/gc.root")

	// Reference labels are used to directly establish a connection
	// from a parent object to a child object. The child object will
	// remain referred to for the lifecycle of the parent object.

	labelGCRef        = []byte("containerd.io/gc.ref.")
	labelGCSnapRef    = []byte("containerd.io/gc.ref.snapshot.")
	labelGCContentRef = []byte("containerd.io/gc.ref.content")
	labelGCImageRef   = []byte("containerd.io/gc.ref.image")

	// Back reference labels are used to establish a reference relationship
	// directly from a child object to a parent object. It allows a child
	// object to attach itself to the lifecycle of a parent without updating
	// the parent object or requiring the parent already exists.

	labelGCContainerBackRef = []byte("containerd.io/gc.bref.container")
	labelGCContentBackRef   = []byte("containerd.io/gc.bref.content")
	labelGCImageBackRef     = []byte("containerd.io/gc.bref.image")
	labelGCSnapBackRef      = []byte("containerd.io/gc.bref.snapshot.")

	// labelGCExpire indicates that an object is collectible after the
	// provided time. For image objects, this makes them available to
	// garbage collect when expired, when not provided, image objects
	// are root objects that never expire. For non-root objects such
	// as content or snapshots, these objects will be treated like
	// root objects before their expiration.
	// Expected format is RFC 3339
	labelGCExpire = []byte("containerd.io/gc.expire")

	// labelGCFlat indicates that a lease is flat and only intends to
	// lease the referenced objects, not their references. This can be
	// used to avoid leasing an entire tree of objects when only the root
	// object is needed.
	labelGCFlat = []byte("containerd.io/gc.flat")
)

// CollectionContext manages a resource collection during a single run of
// the garbage collector. The context is responsible for managing access to
// resources as well as tracking removal.
// Implementations should defer any longer running operations to the Finish
// function and optimize other functions for running fast during garbage
// collection write locks.
type CollectionContext interface {
	// All sends all known resources
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

type collectionWithBackRefs interface {
	ActiveWithBackRefs(namespace string, fn func(gc.Node), bref func(gc.Node, gc.Node))
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
	backRefs      map[gc.Node][]gc.Node
}

type referenceLabelHandler struct {
	key []byte

	// functions to handle reference labels, only one may be set, if none are set
	// the label is triggers the root callback
	fn   func(string, []byte, []byte, func(gc.Node))
	bref func(string, []byte, []byte, func(gc.Node))
}

func startGCContext(ctx context.Context, collectors map[gc.ResourceType]Collector) *gcContext {
	var contexts map[gc.ResourceType]CollectionContext
	labelHandlers := []referenceLabelHandler{
		{
			key: labelGCContainerBackRef,
			bref: func(ns string, k, v []byte, fn func(gc.Node)) {
				if ks := string(k); ks != string(labelGCContainerBackRef) {
					// Allow reference naming separated by . or /, ignore names
					if ks[len(labelGCContainerBackRef)] != '.' && ks[len(labelGCContainerBackRef)] != '/' {
						return
					}
				}

				fn(gcnode(ResourceContainer, ns, string(v)))
			},
		},
		{
			key: labelGCContentBackRef,
			bref: func(ns string, k, v []byte, fn func(gc.Node)) {
				if ks := string(k); ks != string(labelGCContentBackRef) {
					// Allow reference naming separated by . or /, ignore names
					if ks[len(labelGCContentBackRef)] != '.' && ks[len(labelGCContentBackRef)] != '/' {
						return
					}
				}

				fn(gcnode(ResourceContent, ns, string(v)))
			},
		},
		{
			key: labelGCImageBackRef,
			bref: func(ns string, k, v []byte, fn func(gc.Node)) {
				if ks := string(k); ks != string(labelGCImageBackRef) {
					// Allow reference naming separated by . or /, ignore names
					if ks[len(labelGCImageBackRef)] != '.' && ks[len(labelGCImageBackRef)] != '/' {
						return
					}
				}

				fn(gcnode(ResourceImage, ns, string(v)))
			},
		},
		{
			key: labelGCSnapBackRef,
			bref: func(ns string, k, v []byte, fn func(gc.Node)) {
				snapshotter := k[len(labelGCSnapBackRef):]
				if i := bytes.IndexByte(snapshotter, '/'); i >= 0 {
					snapshotter = snapshotter[:i]
				}
				fn(gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", snapshotter, v)))
			},
		},
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
			key: labelGCImageRef,
			fn: func(ns string, k, v []byte, fn func(gc.Node)) {
				if ks := string(k); ks != string(labelGCImageRef) {
					// Allow reference naming separated by . or /, ignore names
					if ks[len(labelGCImageRef)] != '.' && ks[len(labelGCImageRef)] != '/' {
						return
					}
				}

				fn(gcnode(ResourceImage, ns, string(v)))
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
		{
			key: labelGCRoot,
		},
	}
	if len(collectors) > 0 {
		contexts = map[gc.ResourceType]CollectionContext{}
		for rt, collector := range collectors {
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
		// Sort labelHandlers to ensure key seeking is always forward
		sort.Slice(labelHandlers, func(i, j int) bool {
			return bytes.Compare(labelHandlers[i].key, labelHandlers[j].key) < 0
		})
	}
	return &gcContext{
		labelHandlers: labelHandlers,
		contexts:      contexts,
		backRefs:      make(map[gc.Node][]gc.Node),
	}
}

func (c *gcContext) all(fn func(gc.Node)) {
	for _, gctx := range c.contexts {
		gctx.All(fn)
	}
}

func (c *gcContext) active(namespace string, fn func(gc.Node), bref func(gc.Node, gc.Node)) {
	for _, gctx := range c.contexts {
		if cc, ok := gctx.(collectionWithBackRefs); ok {
			// If the context supports back references, use it
			cc.ActiveWithBackRefs(namespace, fn, bref)
		} else {
			// Check if supports back references, if so, add
			gctx.Active(namespace, fn)
		}
	}
}

func (c *gcContext) leased(namespace, lease string, fn func(gc.Node)) {
	for _, gctx := range c.contexts {
		gctx.Leased(namespace, lease, fn)
	}
}

func (c *gcContext) cancel(ctx context.Context) {
	for t, gctx := range c.contexts {
		if err := gctx.Cancel(); err != nil {
			log.G(ctx).WithField("type", t).WithError(err).Error("failed to cancel collection context")
		}
	}
}

func (c *gcContext) finish(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(len(c.contexts))
	for t, gctx := range c.contexts {
		go func() {
			if err := gctx.Finish(); err != nil {
				log.G(ctx).WithField("type", t).WithError(err).Error("failed to finish collection context")
			}
			wg.Done()
		}()
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

	bref := func(n gc.Node, ref gc.Node) {
		if _, ok := c.backRefs[n]; !ok {
			c.backRefs[n] = []gc.Node{ref}
		} else {
			c.backRefs[n] = append(c.backRefs[n], ref)
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
							log.G(ctx).WithField("lease", string(k)).Debug("expired lease")
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

				itype := ResourceImage
				if flat {
					itype = resourceImageFlat
				}

				ibkt = libkt.Bucket(bucketKeyObjectImages)
				if ibkt != nil {
					if err := ibkt.ForEach(func(k, v []byte) error {
						fn(gcnode(itype, ns, string(k)))
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

				if !isExpiredImage(ctx, k, ibkt.Bucket(k), expThreshold) {
					fn(gcnode(ResourceImage, ns, string(k)))
				} else {
					// If the image is expired, still allow it to be referenced from
					// other resources, the back references are not relevant if the object
					// is not expired since it is already a root object.
					return c.sendLabelRefs(ns, ibkt.Bucket(k), nil, func(n gc.Node) {
						bref(n, gcnode(ResourceImage, ns, string(k)))
					}, nil)

				}

				return nil

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

					return c.sendLabelRefs(ns, cbkt.Bucket(k), nil, func(n gc.Node) {
						bref(n, gcnode(ResourceContent, ns, string(k)))
					}, func() {
						fn(gcnode(ResourceContent, ns, string(k)))
					})
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

				fn(gcnode(ResourceContainer, ns, string(k)))

				return c.sendLabelRefs(ns, cbkt.Bucket(k), nil, func(n gc.Node) {
					bref(n, gcnode(ResourceContainer, ns, string(k)))

				}, nil)
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

					return c.sendLabelRefs(ns, snbkt.Bucket(k), nil, func(n gc.Node) {
						bref(n, gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", sk, k)))
					}, func() {
						fn(gcnode(ResourceSnapshot, ns, fmt.Sprintf("%s/%s", sk, k)))
					})
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

				return c.sendLabelRefs(ns, sbbkt, fn, nil, nil)
			}); err != nil {
				return err
			}
		}

		c.active(ns, fn, bref)
	}
	return cerr
}

// references finds the resources that are reachable from the given node.
func (c *gcContext) references(ctx context.Context, tx *bolt.Tx, node gc.Node, fn func(gc.Node)) error {
	if refs, ok := c.backRefs[node]; ok {
		// If we have back references, send them first
		for _, ref := range refs {
			fn(ref)
		}
	}
	switch node.Type {
	case ResourceContent:
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectContent, bucketKeyObjectBlob, []byte(node.Key))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}

		return c.sendLabelRefs(node.Namespace, bkt, fn, nil, nil)
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

		return c.sendLabelRefs(node.Namespace, bkt, fn, nil, nil)

	case ResourceImage, resourceImageFlat:
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectImages, []byte(node.Key))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}
		target := bkt.Bucket(bucketKeyTarget)
		if target != nil {
			ctype := ResourceContent
			if node.Type == resourceImageFlat {
				// For flat leases, keep the target content only
				ctype = resourceContentFlat
			}
			contentKey := string(target.Get(bucketKeyDigest))
			fn(gcnode(ctype, node.Namespace, contentKey))
		}

		// Do not send labeled references for flat image refs
		if node.Type == resourceImageFlat {
			return nil
		}

		return c.sendLabelRefs(node.Namespace, bkt, fn, nil, nil)

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

	case ResourceContainer:
		bkt := getBucket(tx, bucketKeyVersion, []byte(node.Namespace), bucketKeyObjectContainers, []byte(node.Key))
		if bkt == nil {
			// Node may be created from dead edge
			return nil
		}

		snapshotter := string(bkt.Get(bucketKeySnapshotter))
		if snapshotter != "" {
			ss := string(bkt.Get(bucketKeySnapshotKey))
			fn(gcnode(ResourceSnapshot, node.Namespace, fmt.Sprintf("%s/%s", snapshotter, ss)))
		}

		return c.sendLabelRefs(node.Namespace, bkt, fn, nil, nil)
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

		ibkt := nbkt.Bucket(bucketKeyObjectImages)
		if ibkt != nil {
			if err := ibkt.ForEach(func(k, v []byte) error {
				if v != nil {
					return nil
				}
				node := gcnode(ResourceImage, ns, string(k))
				return fn(ctx, node)
			}); err != nil {
				return err
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
	case ResourceImage:
		ibkt := nsbkt.Bucket(bucketKeyObjectImages)
		if ibkt != nil {
			return &eventstypes.ImageDelete{
				Name: node.Key,
			}, ibkt.DeleteBucket([]byte(node.Key))
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
func (c *gcContext) sendLabelRefs(ns string, bkt *bolt.Bucket, fn func(gc.Node), bref func(gc.Node), root func()) error {
	lbkt := bkt.Bucket(bucketKeyObjectLabels)
	if lbkt != nil {
		lc := lbkt.Cursor()
		for i := range c.labelHandlers {
			if (bref == nil && c.labelHandlers[i].bref != nil) || (fn == nil && c.labelHandlers[i].fn != nil) {
				continue
			}
			for k, v := lc.Seek(c.labelHandlers[i].key); k != nil && bytes.HasPrefix(k, c.labelHandlers[i].key); k, v = lc.Next() {
				if c.labelHandlers[i].fn != nil {
					c.labelHandlers[i].fn(ns, k, v, fn)
				} else if c.labelHandlers[i].bref != nil {
					c.labelHandlers[i].bref(ns, k, v, bref)
				} else if root != nil {
					root()
				}
			}
		}
	}
	return nil
}

func isExpiredImage(ctx context.Context, k []byte, bkt *bolt.Bucket, expTheshold time.Time) bool {
	lbkt := bkt.Bucket(bucketKeyObjectLabels)
	if lbkt != nil {
		el := lbkt.Get(labelGCExpire)
		if el != nil {
			exp, err := time.Parse(time.RFC3339, string(el))
			if err != nil {
				log.G(ctx).WithError(err).WithField("image", string(k)).Infof("ignoring invalid expiration value %q", string(el))
				return false
			}
			return expTheshold.After(exp)
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
