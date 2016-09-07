package v1

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/image"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestMakeV1ConfigFromConfig(c *check.C) {
	img := &image.Image{
		V1Image: image.V1Image{
			ID:     "v2id",
			Parent: "v2parent",
			OS:     "os",
		},
		OSVersion: "osversion",
		RootFS: &image.RootFS{
			Type: "layers",
		},
	}
	v2js, err := json.Marshal(img)
	if err != nil {
		c.Fatal(err)
	}

	// Convert the image back in order to get RawJSON() support.
	img, err = image.NewFromJSON(v2js)
	if err != nil {
		c.Fatal(err)
	}

	js, err := MakeV1ConfigFromConfig(img, "v1id", "v1parent", false)
	if err != nil {
		c.Fatal(err)
	}

	newimg := &image.Image{}
	err = json.Unmarshal(js, newimg)
	if err != nil {
		c.Fatal(err)
	}

	if newimg.V1Image.ID != "v1id" || newimg.Parent != "v1parent" {
		c.Error("ids should have changed", newimg.V1Image.ID, newimg.V1Image.Parent)
	}

	if newimg.RootFS != nil {
		c.Error("rootfs should have been removed")
	}

	if newimg.V1Image.OS != "os" {
		c.Error("os should have been preserved")
	}
}
