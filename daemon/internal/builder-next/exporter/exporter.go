package exporter

import "time"

const Moby = "moby"
const BuildRefLabel = "moby/build.ref."

type BuildRefLabelValue struct {
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}
