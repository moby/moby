package cluster

import (
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type infoMetrics struct {
	c        *Cluster
	managers *prometheus.Desc
	nodes    *prometheus.Desc
	info     *prometheus.Desc
}

func newInfoMetrics(c *Cluster, ns *metrics.Namespace) *infoMetrics {
	im := &infoMetrics{
		c:        c,
		managers: ns.NewDesc("managers", "Number of managers in the swarm", metrics.Total),
		nodes:    ns.NewDesc("nodes", "Number of nodes in the swarm", metrics.Total),
		info: ns.NewDesc("info", "Information related to the cluster", metrics.Unit("info"),
			"cluster_id",
			"node_id",
		),
	}
	ns.Add(im)
	return im
}

func (im *infoMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- im.managers
	ch <- im.nodes
	ch <- im.info
}

func (im *infoMetrics) Collect(ch chan<- prometheus.Metric) {
	info := im.c.Info()

	if info.Cluster == nil || info.NodeID == "" {
		return
	}

	ch <- prometheus.MustNewConstMetric(im.managers, prometheus.GaugeValue, float64(info.Managers))
	ch <- prometheus.MustNewConstMetric(im.nodes, prometheus.GaugeValue, float64(info.Nodes))
	ch <- prometheus.MustNewConstMetric(im.info, prometheus.GaugeValue, 1, info.Cluster.ID, info.NodeID)
}
