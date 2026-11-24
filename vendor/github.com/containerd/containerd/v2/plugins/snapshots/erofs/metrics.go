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

package erofs

import "github.com/docker/go-metrics"

var (
	// cacheLookups counts lookups by result. Only layers eligible for the cache
	// are counted, which today means parentless extractions alone.
	cacheLookups metrics.LabeledCounter

	// cacheHits counts hits by the cache that served them (see cacheID), so
	// caches can be told apart. Counted before the snapshot is created, so one
	// that then fails still counts.
	cacheHits metrics.LabeledCounter

	// cacheHitBytes is the size of the blobs served, i.e. blob bytes that didn't
	// have to be built here.
	cacheHitBytes metrics.Counter

	// cacheErrors counts unreadable caches by reason and cache: a down FUSE
	// mount, a permission change since startup. These don't fail the pull, the
	// next cache (or a local conversion) serves the layer instead.
	cacheErrors metrics.LabeledCounter
)

func init() {
	ns := metrics.NewNamespace("containerd", "erofs", nil)

	cacheLookups = ns.NewLabeledCounter("layer_cache_lookups", "layer content cache lookups by result", "result")
	cacheHits = ns.NewLabeledCounter("layer_cache_hits", "layer content cache hits by serving cache", "cache")
	cacheHitBytes = ns.NewCounter("layer_cache_hit_bytes", "bytes of layer blobs served from a layer content cache")
	cacheErrors = ns.NewLabeledCounter("layer_cache_errors", "layer content cache lookup errors", "reason", "cache")

	metrics.Register(ns)
}
