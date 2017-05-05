package daemon

import (
	"sync"

	"github.com/docker/docker/container"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	daemonMetrics *metrics.Namespace

	containerActions          metrics.LabeledTimer
	imageActions              metrics.LabeledTimer
	networkActions            metrics.LabeledTimer
	engineVersion             metrics.LabeledGauge
	engineCpus                metrics.Gauge
	engineMemory              metrics.Gauge
	healthChecksCounter       metrics.Counter
	healthChecksFailedCounter metrics.Counter
)

func init() {
	daemonMetrics = metrics.NewNamespace("engine", "daemon", nil)
	containerActions = daemonMetrics.NewLabeledTimer("container_actions", "The number of seconds it takes to process each container action", "action")
	for _, a := range []string{
		"start",
		"changes",
		"commit",
		"create",
		"delete",
	} {
		containerActions.WithValues(a).Update(0)
	}
	networkActions = daemonMetrics.NewLabeledTimer("network_actions", "The number of seconds it takes to process each network action", "action")
	engineVersion = daemonMetrics.NewLabeledGauge("engine", "The version and commit information for the engine process", metrics.Unit("info"),
		"version",
		"commit",
		"architecture",
		"graph_driver",
		"kernel",
		"os",
	)
	engineCpus = daemonMetrics.NewGauge("engine_cpus", "The number of cpus that the host system of the engine has", metrics.Unit("cpus"))
	engineMemory = daemonMetrics.NewGauge("engine_memory", "The number of bytes of memory that the host system of the engine has", metrics.Bytes)
	healthChecksCounter = daemonMetrics.NewCounter("health_checks", "The total number of health checks")
	healthChecksFailedCounter = daemonMetrics.NewCounter("health_checks_failed", "The total number of failed health checks")
	imageActions = daemonMetrics.NewLabeledTimer("image_actions", "The number of seconds it takes to process each image action", "action")
	metrics.Register(daemonMetrics)
}

type storeMetrics struct {
	d                *Daemon
	containerByState *prometheus.Desc
}

func (m *storeMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.containerByState
}

func (m *storeMetrics) Collect(ch chan<- prometheus.Metric) {
	var mu sync.Mutex
	states := make(map[string]int)
	m.d.containers.ApplyAll(func(c *container.Container) {
		mu.Lock()
		states[c.StateString()]++
		mu.Unlock()
	})

	for state, count := range states {
		ch <- prometheus.MustNewConstMetric(m.containerByState, prometheus.GaugeValue, float64(count), state)
	}
}

func (daemon *Daemon) setupMetrics() {
	sm := &storeMetrics{
		d:                daemon,
		containerByState: daemonMetrics.NewDesc("container_by_state", "Number of containers by state", metrics.Total, "state"),
	}
	daemonMetrics.Add(sm)

	// FIXME: this method never returns an error
	info, _ := daemon.SystemInfo()

	engineVersion.WithValues(
		dockerversion.Version,
		dockerversion.GitCommit,
		info.Architecture,
		info.Driver,
		info.KernelVersion,
		info.OperatingSystem,
	).Set(1)
	engineCpus.Set(float64(info.NCPU))
	engineMemory.Set(float64(info.MemTotal))
}
