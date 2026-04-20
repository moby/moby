package mod

import (
	"runtime/debug"
	"testing"
)

func TestModuleVersion(t *testing.T) {
	tests := []struct {
		name        string
		module      string
		biContent   string
		wantVersion string
	}{
		{
			name: "main module in devel mode returns empty string",
			biContent: `
go	go1.20.3
path	github.com/moby/moby/v2/daemon/internal/builder-next/worker
mod	github.com/moby/moby/v2	(devel)
dep	github.com/moby/buildkit	v0.11.5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
=>	github.com/moby/buildkit	v0.12.0	h1:3YO8J4RtmG7elEgaWMb4HgmpS2CfY1QlaOz9nwB+ZSs=
			`,
			module:      "github.com/moby/moby/v2",
			wantVersion: "",
		},
		{
			name: "main module returns tagged version",
			biContent: `
go	go1.25.8
path	github.com/moby/moby/v2/daemon/internal/mod/gen
mod	github.com/moby/moby/v2	v2.0.0-beta.7
build	-buildmode=exe
build	-compiler=gc
build	CGO_ENABLED=1
build	CGO_CFLAGS=
build	CGO_CPPFLAGS=
build	CGO_CXXFLAGS=
build	CGO_LDFLAGS=
build	GOARCH=arm64
build	GOOS=linux
build	GOARM64=v8.0
build	vcs=git
build	vcs.revision=83bca512aa7ffc1bb4f37ce1107e0d3e3489ad43
build	vcs.time=2026-03-05T14:05:47Z
build	vcs.modified=false
			`,
			module:      "github.com/moby/moby/v2",
			wantVersion: "v2.0.0-beta.7",
		},
		{
			name: "main module returns the base version of pseudo version",
			biContent: `
go	go1.25.8
path	github.com/moby/moby/v2/daemon/internal/mod/gen
mod	github.com/moby/moby/v2	v2.0.0-beta.7.0.20260312170906-aac47873cb5c
build	-buildmode=exe
build	-compiler=gc
build	CGO_ENABLED=1
build	CGO_CFLAGS=
build	CGO_CPPFLAGS=
build	CGO_CXXFLAGS=
build	CGO_LDFLAGS=
build	GOARCH=arm64
build	GOOS=linux
build	GOARM64=v8.0
build	vcs=git
build	vcs.revision=aac47873cb5c31561169c069dba48193ddcbd45c
build	vcs.time=2026-03-12T17:09:06Z
build	vcs.modified=false
`,
			module:      "github.com/moby/moby/v2",
			wantVersion: "v2.0.0-beta.7+aac47873cb5c",
		},
		{
			name: "main module git dirty",
			biContent: `
go	go1.25.8
path	github.com/moby/moby/v2/daemon/internal/mod/gen
mod	github.com/moby/moby/v2	v2.0.0-beta.7.0.20260312170906-aac47873cb5c+dirty
build	-buildmode=exe
build	-compiler=gc
build	CGO_ENABLED=1
build	CGO_CFLAGS=
build	CGO_CPPFLAGS=
build	CGO_CXXFLAGS=
build	CGO_LDFLAGS=
build	GOARCH=arm64
build	GOOS=linux
build	GOARM64=v8.0
build	vcs=git
build	vcs.revision=aac47873cb5c31561169c069dba48193ddcbd45c
build	vcs.time=2026-03-12T17:09:06Z
build	vcs.modified=true
`,
			module:      "github.com/moby/moby/v2",
			wantVersion: "v2.0.0-beta.7+aac47873cb5c+dirty",
		},
		{
			name: "returns empty string if build information not available",
			biContent: `
go	go1.20.3
path	github.com/moby/moby/v2/daemon/internal/builder-next/worker
mod	github.com/moby/moby/v2	(devel)
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "",
		},
		{
			name: "returns the version of buildkit dependency",
			biContent: `
go	go1.20.3
path	github.com/moby/moby/v2/daemon/internal/builder-next/worker
mod	github.com/moby/moby/v2	(devel)
dep	github.com/moby/buildkit	v0.11.5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "v0.11.5",
		},
		{
			name: "returns the replaced version of buildkit dependency",
			biContent: `
go	go1.20.3
path	github.com/moby/moby/v2/daemon/internal/builder-next/worker
mod	github.com/moby/moby/v2	(devel)
dep	github.com/moby/buildkit	v0.11.5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
=>	github.com/moby/buildkit	v0.12.0	h1:3YO8J4RtmG7elEgaWMb4HgmpS2CfY1QlaOz9nwB+ZSs=
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "v0.12.0",
		},
		{
			name: "returns the base version of pseudo version",
			biContent: `
go	go1.20.3
path	github.com/moby/moby/v2/daemon/internal/builder-next/worker
mod	github.com/moby/moby/v2	(devel)
dep	github.com/moby/buildkit	v0.10.7-0.20230306143919-70f2ad56d3e5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "v0.10.6+70f2ad56d3e5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bi, err := debug.ParseBuildInfo(tc.biContent)
			if err != nil {
				t.Fatalf("failed to parse build info: %v", err)
			}
			if gotVersion := moduleVersion(tc.module, bi); gotVersion != tc.wantVersion {
				t.Errorf("moduleVersion() = %v, want %v", gotVersion, tc.wantVersion)
			}
		})
	}
}
