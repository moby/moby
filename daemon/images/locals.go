package images // import "github.com/docker/docker/daemon/images"

import (
	"fmt"

	metrics "github.com/docker/go-metrics"
)

type invalidFilter struct {
	filter string
	value  interface{}
}

func (e invalidFilter) Error() string {
	msg := "Invalid filter '" + e.filter
	if e.value != nil {
		msg += fmt.Sprintf("=%s", e.value)
	}
	return msg + "'"
}

func (e invalidFilter) InvalidParameter() {}

var imageActions metrics.LabeledTimer

func init() {
	ns := metrics.NewNamespace("engine", "daemon", nil)
	imageActions = ns.NewLabeledTimer("image_actions", "The number of seconds it takes to process each image action", "action")
	// TODO: is it OK to register a namespace with the same name? Or does this
	// need to be exported from somewhere?
	metrics.Register(ns)
}
