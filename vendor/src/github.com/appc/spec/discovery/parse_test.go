package discovery

import (
	"reflect"
	"testing"
)

func TestNewAppFromString(t *testing.T) {
	tests := []struct {
		in string

		w    *App
		werr bool
	}{
		{
			"example.com/reduce-worker:1.0.0",

			&App{
				Name: "example.com/reduce-worker",
				Labels: map[string]string{
					"version": "1.0.0",
				},
			},
			false,
		},
		{
			"example.com/reduce-worker,channel=alpha,label=value",

			&App{
				Name: "example.com/reduce-worker",
				Labels: map[string]string{
					"channel": "alpha",
					"label":   "value",
				},
			},

			false,
		},

		// bad AC name for app
		{
			"not an app name",

			nil,
			true,
		},

		// bad URL escape (bad query)
		{
			"example.com/garbage%8 939",

			nil,
			true,
		},

		// multi-value labels
		{
			"foo.com/bar,channel=alpha,dog=woof,channel=beta",

			nil,
			true,
		},
	}
	for i, tt := range tests {
		g, err := NewAppFromString(tt.in)
		gerr := (err != nil)
		if !reflect.DeepEqual(g, tt.w) {
			t.Errorf("#%d: got %v, want %v", i, g, tt.w)
		}
		if gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
	}
}
