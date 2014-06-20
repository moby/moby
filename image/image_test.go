package image

import (
	"encoding/json"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestImageJson(t *testing.T) {
	expect := map[string]string{
		"DockerVersion": "1.0.0-dev",
	}
	fh, err := os.Open("testdata/json.1")
	if err != nil {
		t.Errorf("failed to open test file: %s", err)
	}
	buf, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Errorf("failed to read test file: %s", err)
	}

	oneImage := Image{}

	err = json.Unmarshal(buf, &oneImage)
	if err != nil {
		t.Errorf("failed to unmarshal: %s", err)
	}

	twoImage, err := NewImgJSON(buf)
	if err != nil {
		t.Errorf("encountered an error unmarshaling the image: %s", err)
	}

	if oneImage.DockerVersion != twoImage.DockerVersion {
		t.Errorf("image did not unmarshal correctly")
	}
	if twoImage.DockerVersion != expect["DockerVersion"] {
		t.Errorf("expected DockerVersion: [%s], got [%s]", expect["DockerVersion"], twoImage.DockerVersion)
	}
}

func TestImageMarshal(t *testing.T) {
	startImage := Image{
		ID:            utils.GenerateRandomID(),
		Comment:       "testing",
		Created:       time.Now(),
		DockerVersion: "1.0.0-testing",
	}

	startImageJson, err := json.Marshal(startImage)
	if err != nil {
		t.Errorf("encountered an error marshaling the image: %s", err)
	}

	endImage, err := NewImgJSON(startImageJson)
	if err != nil {
		t.Errorf("encountered an error unmarshaling the image: %s", err)
	}
	if startImage.DockerVersion != endImage.DockerVersion {
		t.Errorf("image did not unmarshal correctly")
	}
}
