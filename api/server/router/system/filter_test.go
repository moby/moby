package system

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"
)

func TestAcceptedEventFilters(t *testing.T) {
	containerFilter := filters.NewArgs()
	containerFilter.Add("container", "dasf3423dags")

	daemonFilter := filters.NewArgs()
	daemonFilter.Add("daemon", "adsf324")

	eventFilter := filters.NewArgs()
	eventFilter.Add("event", "die")

	imageFilter := filters.NewArgs()
	imageFilter.Add("image", "sadsfwewre")

	labelFilter := filters.NewArgs()
	labelFilter.Add("label", "a=b")
	labelFilter.Add("label", "c")

	networkFilter := filters.NewArgs()
	networkFilter.Add("network", "bridge")

	pluginFilter := filters.NewArgs()
	pluginFilter.Add("plugin", "bridge")

	typeFilter := filters.NewArgs()
	typeFilter.Add("type", "container")

	volumeFilter := filters.NewArgs()
	volumeFilter.Add("volume", "asdwqfs34s")

	validEventFilters := []filters.Args{
		containerFilter,
		daemonFilter,
		eventFilter,
		imageFilter,
		labelFilter,
		networkFilter,
		pluginFilter,
		typeFilter,
		volumeFilter,
	}

	for _, singleFilter := range validEventFilters {
		err := singleFilter.Validate(acceptedEventFilters)
		assert.NoError(t, err)
	}

	invalidFilter := filters.NewArgs()
	invalidFilter.Add("nonexisttype", "abcd")

	err := invalidFilter.Validate(acceptedEventFilters)
	expectedErr := fmt.Errorf("Invalid filter 'nonexisttype'")
	assert.EqualError(t, err, expectedErr.Error())
}
