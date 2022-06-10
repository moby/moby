package termtest // import "github.com/docker/docker/integration/internal/termtest"

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestStripANSICommands(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{
			input: "\x1b[2J\x1b[?25l\x1b[m\x1b[Hthis is fine\b\x1b]0;C:\\bin\\sh.exe\x00\a\x1b[?25h\x1b[Ht\x1b[1;13H\x1b[?25laccidents happen \b\x1b[?25h\x1b[Ht\x1b[1;29H",
			want:  "this is fineaccidents happen",
		},
		{
			input: "\x1b[2J\x1b[m\x1b[Hthis is fine\x1b]0;C:\\bin\\sh.exe\a\x1b[?25haccidents happen",
			want:  "this is fineaccidents happen",
		},
	} {
		t.Run("", func(t *testing.T) {
			got, err := StripANSICommands(tt.input)
			assert.NilError(t, err)
			assert.DeepEqual(t, tt.want, got)
		})
	}
}
