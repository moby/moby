package internalshared

import "github.com/moby/moby/api/types/container"

type StreamItem struct {
	Stats *container.StatsResponse
	Error error
}
