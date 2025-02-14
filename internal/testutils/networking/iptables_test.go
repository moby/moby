package networking

import (
	"reflect"
	"testing"
	"time"
)

func Test_getTimeFromLogMsg(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "valid time",
			s:       `time="2025-07-15T13:46:13.414214418Z" level=info msg=""`,
			want:    time.Date(2025, 7, 15, 13, 46, 13, 414214418, time.UTC),
			wantErr: false,
		},
		{
			name:    "invalid format",
			s:       `time="invalid-time-format" level=info msg=""`,
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "missing time",
			s:       `level=info msg=""`,
			want:    time.Time{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractLogTime(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("getTimeFromLogMsg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTimeFromLogMsg() got = %v, want %v", got, tt.want)
			}
		})
	}
}
