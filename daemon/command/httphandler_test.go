package command

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGrpcService(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "package service and method",
			path: "/pkg.Service/Method",
			want: "pkg.Service",
		},
		{
			name: "dotted package path",
			path: "/a.b.C/M",
			want: "a.b.C",
		},
		{
			name: "no method segment",
			path: "/pkg.Service",
			want: "pkg.Service",
		},
		{
			name: "no leading slash",
			path: "pkg.Service/Method",
			want: "pkg.Service",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "only leading slash",
			path: "/",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, grpcService(tc.path), tc.want)
		})
	}
}
