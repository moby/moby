package daemon

import "testing"

// TestValidateCheckpointID checks that IDs which could escape the checkpoint
// directory are rejected, while plain checkpoint names are accepted.
func TestValidateCheckpointID(t *testing.T) {
	for _, id := range []string{"..", "../escape", "foo/../..", "/abs", "bad name", ""} {
		if err := validateCheckpointID(id); err == nil {
			t.Errorf("checkpoint ID %q: expected error, got nil", id)
		}
	}
	for _, id := range []string{"good", "good.name", "good-name_1"} {
		if err := validateCheckpointID(id); err != nil {
			t.Errorf("checkpoint ID %q: unexpected error: %v", id, err)
		}
	}
}
