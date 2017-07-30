package fluentd

import (
	"reflect"
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestInfoMap(t *testing.T) {
	for _, test := range []struct {
		desc  string
		info  logger.Info
		extra map[string]string
	}{
		{
			desc: "default info",
			info: logger.Info{
				Config: map[string]string{
					"info": "",
				},
				ContainerID:   "abcdef",
				ContainerName: "interesting-lastname",
			},
			extra: map[string]string{
				"container_id":   "abcdef",
				"container_name": "interesting-lastname",
			},
		},
		{
			desc: "unknown info attribute",
			info: logger.Info{
				Config: map[string]string{
					"info": "unknownField",
				},
			},
			extra: make(map[string]string),
		},
		{
			desc: "single info",
			info: logger.Info{
				Config: map[string]string{
					"info": "container_name",
				},
				ContainerName: "interesting-lastname",
			},
			extra: map[string]string{
				"container_name": "interesting-lastname",
			},
		},
		{
			desc: "multiple info",
			info: logger.Info{
				Config: map[string]string{
					"info": "container_name,imageName",
				},
				ContainerName:      "interesting-lastname",
				ContainerImageName: "library/hello-world:latest",
			},
			extra: map[string]string{
				"container_name": "interesting-lastname",
				"imageName":      "library/hello-world:latest",
			},
		},
		{
			desc: "multiple info ignore not found",
			info: logger.Info{
				Config: map[string]string{
					"info": "container_name,unknownField,imageName",
				},
				ContainerName:      "interesting-lastname",
				ContainerImageName: "library/hello-world:latest",
			},
			extra: map[string]string{
				"container_name": "interesting-lastname",
				"imageName":      "library/hello-world:latest",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			extra := infoMap(test.info)

			if !reflect.DeepEqual(extra, test.extra) {
				t.Errorf("got info map %#v, wanted %#v", extra, test.extra)
			}
		})
	}
}
