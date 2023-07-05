package overlayutils

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAppendVNIList(t *testing.T) {
	cases := []struct {
		name    string
		slice   []uint32
		csv     string
		want    []uint32
		wantErr string
	}{
		{
			name: "NilSlice",
			csv:  "1,2,3",
			want: []uint32{1, 2, 3},
		},
		{
			name:    "TrailingComma",
			csv:     "1,2,3,",
			want:    []uint32{1, 2, 3},
			wantErr: `invalid vxlan id value "" passed`,
		},
		{
			name:  "EmptySlice",
			slice: make([]uint32, 0, 10),
			csv:   "1,2,3",
			want:  []uint32{1, 2, 3},
		},
		{
			name:  "ExistingSlice",
			slice: []uint32{4, 5, 6},
			csv:   "1,2,3",
			want:  []uint32{4, 5, 6, 1, 2, 3},
		},
		{
			name:    "InvalidVNI",
			slice:   []uint32{4, 5, 6},
			csv:     "1,2,3,abc",
			want:    []uint32{4, 5, 6, 1, 2, 3},
			wantErr: "invalid vxlan id value \"abc\" passed",
		},
		{
			name:    "InvalidVNI2",
			slice:   []uint32{4, 5, 6},
			csv:     "abc,1,2,3",
			want:    []uint32{4, 5, 6},
			wantErr: "invalid vxlan id value \"abc\" passed",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppendVNIList(tt.slice, tt.csv)
			assert.Check(t, is.DeepEqual(tt.want, got))
			if tt.wantErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.ErrorContains(err, tt.wantErr))
			}
		})
	}
}
