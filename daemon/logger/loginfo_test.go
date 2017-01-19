package logger

import (
	"reflect"
	"testing"
)

func TestExtraAttributes(t *testing.T) {
	for _, test := range []struct {
		desc   string
		info   Info
		keyMod func(string) string
		extra  map[string]string
	}{
		{
			desc:  "empty",
			extra: make(map[string]string),
		},
		{
			desc: "empty label attribute",
			info: Info{
				Config: map[string]string{
					"labels": "",
				},
			},
			extra: make(map[string]string),
		},
		{
			desc: "single unknown label",
			info: Info{
				Config: map[string]string{
					"labels": "label1",
				},
			},
			extra: make(map[string]string),
		},
		{
			desc: "single label",
			info: Info{
				Config: map[string]string{
					"labels": "label1",
				},
				ContainerLabels: map[string]string{
					"label1": "value1",
				},
			},
			extra: map[string]string{
				"label1": "value1",
			},
		},
		{
			desc: "single label with mod",
			info: Info{
				Config: map[string]string{
					"labels": "label1",
				},
				ContainerLabels: map[string]string{
					"label1": "value1",
				},
			},
			keyMod: func(string) string {
				return "mod"
			},
			extra: map[string]string{
				"mod": "value1",
			},
		},
		{
			desc: "multi label",
			info: Info{
				Config: map[string]string{
					"labels": "label1,label2",
				},
				ContainerLabels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
			extra: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		{
			desc: "multi label ignore not found",
			info: Info{
				Config: map[string]string{
					"labels": "label1,label2,label3",
				},
				ContainerLabels: map[string]string{
					"label1": "value1",
					"label3": "value3",
				},
			},
			extra: map[string]string{
				"label1": "value1",
				"label3": "value3",
			},
		},
		{
			desc: "empty environment attribute",
			info: Info{
				Config: map[string]string{
					"env": "",
				},
			},
			extra: make(map[string]string),
		},
		{
			desc: "single unknown env var",
			info: Info{
				Config: map[string]string{
					"env": "env1",
				},
			},
			extra: make(map[string]string),
		},
		{
			desc: "single env var",
			info: Info{
				Config: map[string]string{
					"env": "env1",
				},
				ContainerEnv: []string{
					"env1=value1",
				},
			},
			extra: map[string]string{
				"env1": "value1",
			},
		},
		{
			desc: "single env var with mod",
			info: Info{
				Config: map[string]string{
					"env": "env1",
				},
				ContainerEnv: []string{
					"env1=value1",
				},
			},
			keyMod: func(string) string {
				return "mod"
			},
			extra: map[string]string{
				"mod": "value1",
			},
		},
		{
			desc: "multi env var",
			info: Info{
				Config: map[string]string{
					"env": "env1,env2",
				},
				ContainerEnv: []string{
					"env1=value1",
					"env2=value2",
				},
			},
			extra: map[string]string{
				"env1": "value1",
				"env2": "value2",
			},
		},
		{
			desc: "multi env var ignore not found",
			info: Info{
				Config: map[string]string{
					"env": "env1,env2,env3",
				},
				ContainerEnv: []string{
					"env1=value1",
					"env3=value3",
				},
			},
			extra: map[string]string{
				"env1": "value1",
				"env3": "value3",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			extra, _ := test.info.ExtraAttributes(test.keyMod)

			if !reflect.DeepEqual(extra, test.extra) {
				t.Errorf("got extra %#v, wanted %#v", extra, test.extra)
			}
		})
	}
}
