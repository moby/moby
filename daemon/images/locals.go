package images // import "github.com/docker/docker/daemon/images"

import (
	metrics "github.com/docker/go-metrics"
)

var (
	imageActions      metrics.LabeledTimer
	imagePullsStarted metrics.Counter
	imagePulls        metrics.LabeledCounter
)

func init() {
	ns := metrics.NewNamespace("engine", "daemon", nil)

	// imagePullsStarted increments when a pull starts.
	imagePullsStarted = ns.NewCounter("image_pulls_started", "The number of total image pulls started")
	// imagePulls increments when a pull finishes. By subtracting this from
	// ImagePullsStarted, the user can determine how many pulls are in progress.
	imagePulls = ns.NewLabeledCounter("image_pulls", "The number of total image pulls", "status")

	imageActions = ns.NewLabeledTimer("image_actions", "The number of seconds it takes to process each image action", "action")
	// TODO: is it OK to register a namespace with the same name? Or does this
	// need to be exported from somewhere?
	metrics.Register(ns)
}
