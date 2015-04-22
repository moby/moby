package main

import (
	"encoding/json"
	"net/url"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestLegacyImages(c *check.C) {
	_, body, err := sockRequest("GET", "/v1.6/images/json", nil)
	if err != nil {
		c.Fatalf("Error on GET: %s", err)
	}

	images := []types.LegacyImage{}
	if err = json.Unmarshal(body, &images); err != nil {
		c.Fatalf("Error on unmarshal: %s", err)
	}

	if len(images) == 0 || images[0].Tag == "" || images[0].Repository == "" {
		c.Fatalf("Bad data: %q", images)
	}
}

func (s *DockerSuite) TestApiImagesFilter(c *check.C) {
	name := "utest:tag1"
	name2 := "utest/docker:tag2"
	name3 := "utest:5000/docker:tag3"
	defer deleteImages(name, name2, name3)
	for _, n := range []string{name, name2, name3} {
		if out, err := exec.Command(dockerBinary, "tag", "busybox", n).CombinedOutput(); err != nil {
			c.Fatal(err, out)
		}
	}
	type image struct{ RepoTags []string }
	getImages := func(filter string) []image {
		v := url.Values{}
		v.Set("filter", filter)
		_, b, err := sockRequest("GET", "/images/json?"+v.Encode(), nil)
		if err != nil {
			c.Fatal(err)
		}
		var images []image
		if err := json.Unmarshal(b, &images); err != nil {
			c.Fatal(err)
		}

		return images
	}

	errMsg := "incorrect number of matches returned"
	if images := getImages("utest*/*"); len(images[0].RepoTags) != 2 {
		c.Fatal(errMsg)
	}
	if images := getImages("utest"); len(images[0].RepoTags) != 1 {
		c.Fatal(errMsg)
	}
	if images := getImages("utest*"); len(images[0].RepoTags) != 1 {
		c.Fatal(errMsg)
	}
	if images := getImages("*5000*/*"); len(images[0].RepoTags) != 1 {
		c.Fatal(errMsg)
	}
}

func (s *DockerSuite) TestApiImagesSaveAndLoad(c *check.C) {
	testRequires(c, Network)
	out, err := buildImage("saveandload", "FROM hello-world\nENV FOO bar", false)
	if err != nil {
		c.Fatal(err)
	}
	id := strings.TrimSpace(out)
	defer deleteImages("saveandload")

	_, body, err := sockRequestRaw("GET", "/images/"+id+"/get", nil, "")
	if err != nil {
		c.Fatal(err)
	}
	defer body.Close()

	if out, err := exec.Command(dockerBinary, "rmi", id).CombinedOutput(); err != nil {
		c.Fatal(err, out)
	}

	_, loadBody, err := sockRequestRaw("POST", "/images/load", body, "application/x-tar")
	if err != nil {
		c.Fatal(err)
	}
	defer loadBody.Close()

	inspectOut, err := exec.Command(dockerBinary, "inspect", "--format='{{ .Id }}'", id).CombinedOutput()
	if err != nil {
		c.Fatal(err, inspectOut)
	}
	if strings.TrimSpace(string(inspectOut)) != id {
		c.Fatal("load did not work properly")
	}
}
