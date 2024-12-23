package events // import "github.com/docker/docker/daemon/events"

import gometrics "github.com/docker/go-metrics"

var (
	eventsCounter    gometrics.Counter
	eventSubscribers gometrics.Gauge
)

func init() {
	ns := gometrics.NewNamespace("engine", "daemon", nil)
	eventsCounter = ns.NewCounter("events", "The number of events logged")
	eventSubscribers = ns.NewGauge("events_subscribers", "The number of current subscribers to events", gometrics.Total)
	gometrics.Register(ns)
}
