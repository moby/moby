package opts

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNRIOptsJSON(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		expErr string
		expNRI NRIConfig
	}{
		{
			name:   "no config",
			expNRI: NRIConfig{},
		},
		{
			name: "json enable",
			json: `{"enable": true}`,
			expNRI: NRIConfig{
				Enable: true,
			},
		},
		{
			name: "json enable with paths",
			json: `{"enable": true, "plugin-path": "/foo", "plugin-config-path": "/bar", "socket-path": "/baz"}`,
			expNRI: NRIConfig{
				Enable:           true,
				PluginPath:       "/foo",
				PluginConfigPath: "/bar",
				SocketPath:       "/baz",
			},
		},
		{
			name:   "json unknown field",
			json:   `{"foo": "bar"}`,
			expErr: "unknown field \"foo\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var nc NRIConfig
			if tc.json != "" {
				err := nc.UnmarshalJSON([]byte(tc.json))
				if tc.expErr != "" {
					assert.Check(t, is.ErrorContains(err, tc.expErr))
					return
				}
				assert.Check(t, err)
			}
			assert.Check(t, is.Equal(nc, tc.expNRI))
		})
	}
}

func TestNRIOptsCmd(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		expErr    string
		expNRI    NRIConfig
		expString string
	}{
		{
			name: "cmd enable",
			cmd:  "enable=true",
			expNRI: NRIConfig{
				Enable: true,
			},
			expString: "enable=true",
		},
		{
			name: "cmd enable with paths",
			cmd:  "enable=true,plugin-path=/foo,plugin-config-path=/bar,socket-path=/baz",
			expNRI: NRIConfig{
				Enable:           true,
				PluginPath:       "/foo",
				PluginConfigPath: "/bar",
				SocketPath:       "/baz",
			},
			expString: "enable=true,plugin-path=/foo,plugin-config-path=/bar,socket-path=/baz",
		},
		{
			// "--nri enable"
			name: "cmd enable no value",
			cmd:  `enable`,
			expNRI: NRIConfig{
				Enable: true,
			},
			expString: "enable=true",
		},
		{
			name:   "cmd unknown field",
			cmd:    `foo=bar`,
			expErr: "unexpected key 'foo' in 'foo=bar'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var nc NRIConfig
			nriOpt := NewNamedNRIOptsRef(&nc)
			err := nriOpt.Set(tc.cmd)
			if tc.expErr != "" {
				assert.Check(t, is.ErrorContains(err, tc.expErr))
				return
			}
			assert.Check(t, err)
			assert.Check(t, is.Equal(nriOpt.String(), tc.expString))
		})
	}
}
