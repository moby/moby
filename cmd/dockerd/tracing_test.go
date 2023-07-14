package main

import (
	"context"
	"net"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSetupTracing(t *testing.T) {
	type testCase struct {
		env map[string]string

		xDisabled    bool
		xErrContains string
	}

	// Create a dummy listener just so the otlp lib has something to connect to and it is not some random service on the host
	l, err := net.Listen("tcp", "localhost:0")
	assert.NilError(t, err)
	defer l.Close()
	addr := l.Addr().String()

	testCases := []testCase{
		{xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "true"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "1"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "0"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "false"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "invalid"}, xDisabled: true},

		{env: map[string]string{otelExporterOTLPEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelSDKDisabledEnv: "1", otelExporterOTLPEndpointEnv: addr}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "0", otelExporterOTLPEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelSDKDisabledEnv: "true", otelExporterOTLPEndpointEnv: addr}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "false", otelExporterOTLPEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelSDKDisabledEnv: "invalid", otelExporterOTLPEndpointEnv: addr}, xDisabled: true},

		{env: map[string]string{otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelExporterOTLPTracesEndpointEnv: addr, otelExporterOTLPProtocolEnv: "invalid"}, xErrContains: "unsupported otlp protocol"},
		{env: map[string]string{otelSDKDisabledEnv: "1", otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "0", otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelSDKDisabledEnv: "0", otelExporterOTLPTracesEndpointEnv: addr, otelExporterOTLPProtocolEnv: "invalid"}, xErrContains: "unsupported otlp protocol"},
		{env: map[string]string{otelSDKDisabledEnv: "true", otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "false", otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: false},
		{env: map[string]string{otelSDKDisabledEnv: "invalid", otelExporterOTLPTracesEndpointEnv: addr}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "false", otelExporterOTLPTracesEndpointEnv: addr, otelTracesExporterEnv: "none"}, xDisabled: true},

		{env: map[string]string{otelTracesExporterEnv: "otlp"}, xDisabled: false},
		{env: map[string]string{otelTracesExporterEnv: "none"}, xDisabled: true},
		{env: map[string]string{otelTracesExporterEnv: "invalid"}, xErrContains: otelTracesExporterEnv},
		{env: map[string]string{otelTracesExporterEnv: "none"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "1", otelTracesExporterEnv: "otlp"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "true", otelTracesExporterEnv: "otlp"}, xDisabled: true},
		{env: map[string]string{otelSDKDisabledEnv: "invalid", otelTracesExporterEnv: "otlp"}, xDisabled: true},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		tc := tc
		t.Run("", func(t *testing.T) {
			getEnv := func(key string) string {
				return tc.env[key]
			}

			defer func() {
				if t.Failed() {
					t.Log(tc)
				}
			}()

			tp, err := getTracerProvider(ctx, getEnv)
			defer func() {
				if tp != nil {
					assert.Check(t, tp.Shutdown(ctx))
				}
			}()

			switch {
			case tc.xDisabled:
				assert.Check(t, is.ErrorIs(err, errTracingDisabled))
				assert.Check(t, is.Nil(tp))
			case tc.xErrContains != "":
				assert.Check(t, is.ErrorContains(err, tc.xErrContains))
				assert.Check(t, is.Nil(tp))
			default:
				assert.Check(t, err)
				assert.Check(t, tp != nil)
			}
		})
	}
}
