package usergroup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/sys/user"
)

func TestParseSubidFileWithNewlinesAndComments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsesubid")
	if err != nil {
		t.Fatal(err)
	}
	fnamePath := filepath.Join(tmpDir, "testsubuid")
	fcontent := `tss:100000:65536
# empty default subuid/subgid file

dockremap:231072:65536`
	if err := os.WriteFile(fnamePath, []byte(fcontent), 0o644); err != nil {
		t.Fatal(err)
	}
	ranges, err := user.ParseSubIDFileFilter(fnamePath, func(sid user.SubID) bool {
		return sid.Name == "dockremap"
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("wanted 1 element in ranges, got %d instead", len(ranges))
	}
	if ranges[0].SubID != 231072 {
		t.Fatalf("wanted 231072, got %d instead", ranges[0].SubID)
	}
	if ranges[0].Count != 65536 {
		t.Fatalf("wanted 65536, got %d instead", ranges[0].Count)
	}
}
