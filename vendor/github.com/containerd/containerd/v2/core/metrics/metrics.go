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

package metrics

import (
	"time"

	"github.com/containerd/containerd/v2/pkg/timeout"
	"github.com/containerd/containerd/v2/version"
	goMetrics "github.com/docker/go-metrics"
)

const (
	ShimStatsRequestTimeout = "io.containerd.timeout.metrics.shimstats"
)

func init() {
	ns := goMetrics.NewNamespace("containerd", "", nil)
	c := ns.NewLabeledCounter("build_info", "containerd build information", "version", "revision")
	c.WithValues(version.Version, version.Revision).Inc()
	goMetrics.Register(ns)
	timeout.Set(ShimStatsRequestTimeout, 2*time.Second)
}
