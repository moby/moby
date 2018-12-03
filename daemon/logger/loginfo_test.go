package logger

import (
	"testing"

	"github.com/docker/docker/errdefs"
)

func TestExtraAttributes(t *testing.T) {
	type testCase struct {
		desc    string
		info    *Info
		expect  map[string]string
		errTest func(*testing.T, error)
	}

	noErr := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	invalidParam := func(t *testing.T, err error) {
		t.Helper()
		if !errdefs.IsInvalidParameter(err) {
			t.Fatalf("expected invalid parameter error, got: %T --  %v", err, err)
		}
	}

	for _, tc := range []testCase{
		{desc: "no extra", info: &Info{}, errTest: noErr},
		{desc: "incorrect field value", info: &Info{Config: map[string]string{logInfoKey: "NotAField"}}, errTest: invalidParam},
		{
			desc: "container env",
			info: &Info{
				Config:       map[string]string{envKey: "FOO,NOTEXIST"},
				ContainerEnv: []string{"FOO=foo", "BAR=bar"},
			},
			expect:  map[string]string{"FOO": "foo"},
			errTest: noErr,
		},
		{
			desc: "container env regex",
			info: &Info{
				Config:       map[string]string{envRegexKey: "(FOO|BAR)"},
				ContainerEnv: []string{"FOO=foo", "BAR=bar", "QUUX=quux"},
			},
			expect:  map[string]string{"FOO": "foo", "BAR": "bar"},
			errTest: noErr,
		},
		{
			desc: "container labels",
			info: &Info{
				Config:          map[string]string{labelKey: "com.docker.test-a-thing,com.docker.test-not-exist"},
				ContainerLabels: map[string]string{"com.docker.test-a-thing": "foo"},
			},
			expect:  map[string]string{"com.docker.test-a-thing": "foo"},
			errTest: noErr,
		},
		{
			desc: "with info field", info: &Info{
				ContainerImageID: "myImage",
				Config:           map[string]string{logInfoKey: "ContainerImageID"},
			},
			expect:  map[string]string{"ContainerImageID": "myImage"},
			errTest: noErr,
		},
		{
			desc: "with multiple info fields",
			info: &Info{
				ContainerImageID:   "myImageID",
				ContainerImageName: "myImageName",
				ContainerID:        "myID",
				Config:             map[string]string{logInfoKey: "ContainerImageID,ContainerImageName,ContainerID"},
			},
			expect:  map[string]string{"ContainerID": "myID", "ContainerImageID": "myImageID", "ContainerImageName": "myImageName"},
			errTest: noErr,
		},
		{
			desc: "all thing things",
			info: &Info{
				ContainerID:        "myID",
				ContainerImageName: "myImage",
				ContainerImageID:   "myImageID",
				ContainerLabels:    map[string]string{"com.docker.test-a-thing": "banana"},
				ContainerEnv:       []string{"FOO=foo", "BAR=bar", "BAZ=baz"},
				Config: map[string]string{
					labelKey:    "com.docker.test-a-thing",
					envKey:      "FOO",
					envRegexKey: "(FOO|BAR)",
					logInfoKey:  "ContainerImageName,ContainerID",
				},
			},
			expect: map[string]string{
				"FOO":                     "foo",
				"com.docker.test-a-thing": "banana",
				"BAR":                     "bar",
				"ContainerImageName":      "myImage",
				"ContainerID":             "myID",
			},
			errTest: noErr,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			tc := tc
			t.Parallel()

			actual, err := tc.info.ExtraAttributes(nil)
			tc.errTest(t, err)

			if len(actual) != len(tc.expect) {
				t.Fatalf("expected:\n%v\n\nactual:\n%v", tc.expect, actual)
			}

			for k, v := range actual {
				ev, ok := tc.expect[k]
				if !ok {
					t.Fatalf("unexpected key: %s", ev)
				}

				if v != ev {
					t.Fatalf("unexpected value for key %q, expected: %q, got: %q", k, ev, v)
				}
			}
		})
	}
}
