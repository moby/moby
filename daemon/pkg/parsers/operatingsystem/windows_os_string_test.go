package operatingsystem

import (
	"testing"
)

func Test_windowsOSRelease_String(t *testing.T) {
	tests := []struct {
		name string
		r    windowsOSRelease
		want string
	}{
		{
			name: "Flavor=client/DisplayVersion=yes/UBR=yes",
			r: windowsOSRelease{
				DisplayVersion: "1809",
				Build:          17763,
				UBR:            2628,
			},
			want: "Microsoft Windows Version 1809 (OS Build 17763.2628)",
		},
		{
			name: "Flavor=client/DisplayVersion=yes/UBR=no",
			r: windowsOSRelease{
				DisplayVersion: "1809",
				Build:          17763,
			},
			want: "Microsoft Windows Version 1809 (OS Build 17763)",
		},
		{
			name: "Flavor=client/DisplayVersion=no/UBR=yes",
			r: windowsOSRelease{
				Build: 17763,
				UBR:   1879,
			},
			want: "Microsoft Windows (OS Build 17763.1879)",
		},
		{
			name: "Flavor=client/DisplayVersion=no/UBR=no",
			r: windowsOSRelease{
				Build: 10240,
			},
			want: "Microsoft Windows (OS Build 10240)",
		},
		{
			name: "Flavor=server/DisplayVersion=yes/UBR=yes",
			r: windowsOSRelease{
				IsServer:       true,
				DisplayVersion: "21H2",
				Build:          20348,
				UBR:            169,
			},
			want: "Microsoft Windows Server Version 21H2 (OS Build 20348.169)",
		},
		{
			name: "Flavor=server/DisplayVersion=yes/UBR=no",
			r: windowsOSRelease{
				IsServer:       true,
				DisplayVersion: "20H2",
				Build:          19042,
			},
			want: "Microsoft Windows Server Version 20H2 (OS Build 19042)",
		},
		{
			name: "Flavor=server/DisplayVersion=no/UBR=yes",
			r: windowsOSRelease{
				IsServer: true,
				Build:    17763,
				UBR:      107,
			},
			want: "Microsoft Windows Server (OS Build 17763.107)",
		},
		{
			name: "Flavor=server/DisplayVersion=no/UBR=no",
			r: windowsOSRelease{
				IsServer: true,
				Build:    17763,
			},
			want: "Microsoft Windows Server (OS Build 17763)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("windowsOSRelease.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
