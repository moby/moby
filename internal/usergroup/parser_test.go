package usergroup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/idtools"
	"gotest.tools/v3/assert"
)

func TestCreateIDMapOrder(t *testing.T) {
	subidRanges := subIDRanges{
		{100000, 1000},
		{1000, 1},
	}

	idMap := createIDMap(subidRanges)
	assert.DeepEqual(t, idMap, []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      100000,
			Size:        1000,
		},
		{
			ContainerID: 1000,
			HostID:      1000,
			Size:        1,
		},
	})
}

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
	ranges, err := parseSubidFile(fnamePath, "dockremap")
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("wanted 1 element in ranges, got %d instead", len(ranges))
	}
	if ranges[0].Start != 231072 {
		t.Fatalf("wanted 231072, got %d instead", ranges[0].Start)
	}
	if ranges[0].Length != 65536 {
		t.Fatalf("wanted 65536, got %d instead", ranges[0].Length)
	}
}
