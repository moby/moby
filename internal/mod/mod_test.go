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
			name: "returns empty string if build information not available",
			biContent: `
go	go1.20.3
path	github.com/docker/docker/builder/builder-next/worker
mod	github.com/docker/docker	(devel)
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "",
		},
		{
			name: "returns the version of buildkit dependency",
			biContent: `
go	go1.20.3
path	github.com/docker/docker/builder/builder-next/worker
mod	github.com/docker/docker	(devel)
dep	github.com/moby/buildkit	v0.11.5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "v0.11.5",
		},
		{
			name: "returns the replaced version of buildkit dependency",
			biContent: `
go	go1.20.3
path	github.com/docker/docker/builder/builder-next/worker
mod	github.com/docker/docker	(devel)
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
path	github.com/docker/docker/builder/builder-next/worker
mod	github.com/docker/docker	(devel)
dep	github.com/moby/buildkit	v0.10.7-0.20230306143919-70f2ad56d3e5	h1:JZvvWzulcnA2G4c/gJiSIqKDUoBjctYw2WMuS+XJexU=
			`,
			module:      "github.com/moby/buildkit",
			wantVersion: "v0.10.6+70f2ad56d3e5",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			bi, err := debug.ParseBuildInfo(tt.biContent)
			if err != nil {
				t.Fatalf("failed to parse build info: %v", err)
			}
			if gotVersion := moduleVersion(tt.module, bi); gotVersion != tt.wantVersion {
				t.Errorf("moduleVersion() = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}
