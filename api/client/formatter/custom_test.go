package formatter

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
)

func TestContainerPsContext(t *testing.T) {
	containerID := stringid.GenerateRandomID()
	unix := time.Now().Unix()

	var ctx containerContext
	cases := []struct {
		container types.Container
		trunc     bool
		expValue  string
		expHeader string
		call      func() string
	}{
		{types.Container{ID: containerID}, true, stringid.TruncateID(containerID), containerIDHeader, ctx.ID},
		{types.Container{ID: containerID}, false, containerID, containerIDHeader, ctx.ID},
		{types.Container{Names: []string{"/foobar_baz"}}, true, "foobar_baz", namesHeader, ctx.Names},
		{types.Container{Image: "ubuntu"}, true, "ubuntu", imageHeader, ctx.Image},
		{types.Container{Image: "verylongimagename"}, true, "verylongimagename", imageHeader, ctx.Image},
		{types.Container{Image: "verylongimagename"}, false, "verylongimagename", imageHeader, ctx.Image},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			true,
			"a5a665ff33ec",
			imageHeader,
			ctx.Image,
		},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			false,
			"a5a665ff33eced1e0803148700880edab4",
			imageHeader,
			ctx.Image,
		},
		{types.Container{Image: ""}, true, "<no image>", imageHeader, ctx.Image},
		{types.Container{Command: "sh -c 'ls -la'"}, true, `"sh -c 'ls -la'"`, commandHeader, ctx.Command},
		{types.Container{Created: unix}, true, time.Unix(unix, 0).String(), createdAtHeader, ctx.CreatedAt},
		{types.Container{Ports: []types.Port{{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"}}}, true, "8080/tcp", portsHeader, ctx.Ports},
		{types.Container{Status: "RUNNING"}, true, "RUNNING", statusHeader, ctx.Status},
		{types.Container{SizeRw: 10}, true, "10 B", sizeHeader, ctx.Size},
		{types.Container{SizeRw: 10, SizeRootFs: 20}, true, "10 B (virtual 20 B)", sizeHeader, ctx.Size},
		{types.Container{}, true, "", labelsHeader, ctx.Labels},
		{types.Container{Labels: map[string]string{"cpu": "6", "storage": "ssd"}}, true, "cpu=6,storage=ssd", labelsHeader, ctx.Labels},
		{types.Container{Created: unix}, true, "Less than a second", runningForHeader, ctx.RunningFor},
	}

	for _, c := range cases {
		ctx = containerContext{c: c.container, trunc: c.trunc}
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}

		h := ctx.fullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}

	c1 := types.Container{Labels: map[string]string{"com.docker.swarm.swarm-id": "33", "com.docker.swarm.node_name": "ubuntu"}}
	ctx = containerContext{c: c1, trunc: true}

	sid := ctx.Label("com.docker.swarm.swarm-id")
	node := ctx.Label("com.docker.swarm.node_name")
	if sid != "33" {
		t.Fatalf("Expected 33, was %s\n", sid)
	}

	if node != "ubuntu" {
		t.Fatalf("Expected ubuntu, was %s\n", node)
	}

	h := ctx.fullHeader()
	if h != "SWARM ID\tNODE NAME" {
		t.Fatalf("Expected %s, was %s\n", "SWARM ID\tNODE NAME", h)

	}

	c2 := types.Container{}
	ctx = containerContext{c: c2, trunc: true}

	label := ctx.Label("anything.really")
	if label != "" {
		t.Fatalf("Expected an empty string, was %s", label)
	}

	ctx = containerContext{c: c2, trunc: true}
	fullHeader := ctx.fullHeader()
	if fullHeader != "" {
		t.Fatalf("Expected fullHeader to be empty, was %s", fullHeader)
	}

}

func TestImagesContext(t *testing.T) {
	imageID := stringid.GenerateRandomID()
	unix := time.Now().Unix()

	var ctx imageContext
	cases := []struct {
		imageCtx  imageContext
		expValue  string
		expHeader string
		call      func() string
	}{
		{imageContext{
			i:     types.Image{ID: imageID},
			trunc: true,
		}, stringid.TruncateID(imageID), imageIDHeader, ctx.ID},
		{imageContext{
			i:     types.Image{ID: imageID},
			trunc: false,
		}, imageID, imageIDHeader, ctx.ID},
		{imageContext{
			i:     types.Image{Size: 10},
			trunc: true,
		}, "10 B", sizeHeader, ctx.Size},
		{imageContext{
			i:     types.Image{Created: unix},
			trunc: true,
		}, time.Unix(unix, 0).String(), createdAtHeader, ctx.CreatedAt},
		// FIXME
		// {imageContext{
		// 	i:     types.Image{Created: unix},
		// 	trunc: true,
		// }, units.HumanDuration(time.Unix(unix, 0)), createdSinceHeader, ctx.CreatedSince},
		{imageContext{
			i:    types.Image{},
			repo: "busybox",
		}, "busybox", repositoryHeader, ctx.Repository},
		{imageContext{
			i:   types.Image{},
			tag: "latest",
		}, "latest", tagHeader, ctx.Tag},
		{imageContext{
			i:      types.Image{},
			digest: "sha256:d149ab53f8718e987c3a3024bb8aa0e2caadf6c0328f1d9d850b2a2a67f2819a",
		}, "sha256:d149ab53f8718e987c3a3024bb8aa0e2caadf6c0328f1d9d850b2a2a67f2819a", digestHeader, ctx.Digest},
	}

	for _, c := range cases {
		ctx = c.imageCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}

		h := ctx.fullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}
}

func compareMultipleValues(t *testing.T, value, expected string) {
	// comma-separated values means probably a map input, which won't
	// be guaranteed to have the same order as our expected value
	// We'll create maps and use reflect.DeepEquals to check instead:
	entriesMap := make(map[string]string)
	expMap := make(map[string]string)
	entries := strings.Split(value, ",")
	expectedEntries := strings.Split(expected, ",")
	for _, entry := range entries {
		keyval := strings.Split(entry, "=")
		entriesMap[keyval[0]] = keyval[1]
	}
	for _, expected := range expectedEntries {
		keyval := strings.Split(expected, "=")
		expMap[keyval[0]] = keyval[1]
	}
	if !reflect.DeepEqual(expMap, entriesMap) {
		t.Fatalf("Expected entries: %v, got: %v", expected, value)
	}
}
