package images // import "github.com/docker/docker/daemon/images"

import (
	gometrics "github.com/docker/go-metrics"
)

// ImageActions measures the time it takes to process some image actions.
// Exported for use in the containerd-backed image store and it is not intended
// for external consumption. Do not use!
var ImageActions gometrics.LabeledTimer

func init() {
	ns := gometrics.NewNamespace("engine", "daemon", nil)
	ImageActions = ns.NewLabeledTimer("image_actions", "The number of seconds it takes to process each image action", "action")
	// TODO: is it OK to register a namespace with the same name? Or does this
	// need to be exported from somewhere?
	gometrics.Register(ns)
}
