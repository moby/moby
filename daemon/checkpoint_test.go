package daemon

import (
	"testing"

	cerrdefs "github.com/containerd/errdefs"
)

// TestValidateCheckpointID checks that IDs which could escape the checkpoint
// directory are rejected, while plain checkpoint names are accepted.
func TestValidateCheckpointID(t *testing.T) {
	for _, id := range []string{"..", "../escape", "foo/../..", "/abs", "bad name", "", "   "} {
		err := validateCheckpointID(id)
		if err == nil {
			t.Errorf("checkpoint ID %q: expected error, got nil", id)
			continue
		}
		if !cerrdefs.IsInvalidArgument(err) {
			t.Errorf("checkpoint ID %q: expected an invalid parameter error, got %v", id, err)
		}
	}
	for _, id := range []string{"good", "good.name", "good-name_1"} {
		if err := validateCheckpointID(id); err != nil {
			t.Errorf("checkpoint ID %q: unexpected error: %v", id, err)
		}
	}
}
