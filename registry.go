package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
)

//FIXME: Set the endpoint in a conf file or via commandline
//const REGISTRY_ENDPOINT = "http://registry-creack.dotcloud.com/v1"
const REGISTRY_ENDPOINT = auth.REGISTRY_SERVER + "/v1"

// Build an Image object from raw json data
func NewImgJson(src []byte) (*Image, error) {
	ret := &Image{}

	fmt.Printf("Json string: {%s}\n", src)
	// FIXME: Is there a cleaner way to "puryfy" the input json?
	src = []byte(strings.Replace(string(src), "null", "\"\"", -1))

	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// Build an Image object list from a raw json data
// FIXME: Do this in "stream" mode
func NewMultipleImgJson(src []byte) ([]*Image, error) {
	ret := []*Image{}

	dec := json.NewDecoder(strings.NewReader(strings.Replace(string(src), "null", "\"\"", -1)))
	for {
		m := &Image{}
		if err := dec.Decode(m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		ret = append(ret, m)
	}
	return ret, nil
}

// Retrieve the history of a given image from the Registry.
// Return a list of the parent's json (requested image included)
func (graph *Graph) getRemoteHistory(imgId string, authConfig *auth.AuthConfig) ([]*Image, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/images/"+imgId+"/history", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Error while reading the http response: %s\n", err)
	}

	history, err := NewMultipleImgJson(jsonString)
	if err != nil {
		return nil, fmt.Errorf("Error while parsing the json: %s\n", err)
	}
	return history, nil
}

// Check if an image exists in the Registry
func (graph *Graph) LookupRemoteImage(imgId string, authConfig *auth.AuthConfig) bool {
	client := &http.Client{}

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/images/"+imgId+"/json", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	return res.StatusCode == 307
}

// Retrieve an image from the Registry.
// Returns the Image object as well as the layer as an Archive (io.Reader)
func (graph *Graph) getRemoteImage(imgId string, authConfig *auth.AuthConfig) (*Image, Archive, error) {
	client := &http.Client{}

	// Get the Json
	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/images/"+imgId+"/json", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("Error while reading the http response: %s\n", err)
	}

	img, err := NewImgJson(jsonString)
	if err != nil {
		return nil, nil, fmt.Errorf("Error while parsing the json: %s\n", err)
	}
	img.Id = imgId

	// Get the layer
	res, err = http.Get(REGISTRY_ENDPOINT + "/images/" + imgId + "/layer")
	if err != nil {
		return nil, nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	return img, res.Body, nil
}

func (graph *Graph) PullImage(imgId string, authConfig *auth.AuthConfig) error {
	history, err := graph.getRemoteHistory(imgId, authConfig)
	if err != nil {
		return err
	}
	// FIXME: Try to stream the images?
	// FIXME: Lunch the getRemoteImage() in goroutines
	for _, j := range history {
		if !graph.Exists(j.Id) {
			img, layer, err := graph.getRemoteImage(j.Id, authConfig)
			if err != nil {
				// FIXME: Keep goging in case of error?
				return err
			}
			if err = graph.Register(layer, img); err != nil {
				return err
			}
		}
	}
	return nil
}

// FIXME: Handle the askedTag parameter
func (graph *Graph) PullRepository(user, repoName, askedTag string, repositories *TagStore, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/users/"+user+"/"+repoName, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	rawJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	t := map[string]string{}
	if err = json.Unmarshal(rawJson, &t); err != nil {
		return err
	}
	for tag, rev := range t {
		if err = graph.PullImage(rev, authConfig); err != nil {
			return err
		}
		if err = repositories.Set(repoName, tag, rev, true); err != nil {
			return err
		}
	}
	if err = repositories.Save(); err != nil {
		return err
	}
	return nil
}

// Push a local image to the registry with its history if needed
func (graph *Graph) PushImage(imgOrig *Image, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	// FIXME: Factorize the code
	// FIXME: Do the puts in goroutines
	if err := imgOrig.WalkHistory(func(img *Image) error {

		jsonRaw, err := ioutil.ReadFile(path.Join(graph.Root, img.Id, "json"))
		if err != nil {
			return fmt.Errorf("Error while retreiving the path for {%s}: %s", img.Id, err)
		}
		// FIXME: try json with UTF8
		jsonData := strings.NewReader(string(jsonRaw))
		req, err := http.NewRequest("PUT", REGISTRY_ENDPOINT+"/images/"+img.Id+"/json", jsonData)
		if err != nil {
			return err
		}
		req.Header.Add("Content-type", "application/json")
		req.SetBasicAuth(authConfig.Username, authConfig.Password)
		res, err := client.Do(req)
		if err != nil || res.StatusCode != 200 {
			if res == nil {
				return fmt.Errorf(
					"Error: Internal server error trying to push image {%s} (json): %s",
					img.Id, err)
			}
			switch res.StatusCode {
			case 204:
				// Case where the image is already on the Registry
				// FIXME: Do not be silent?
				return nil
			case 400:
				return fmt.Errorf("Error: Invalid Json")
			default:
				return fmt.Errorf(
					"Error: Internal server error trying to push image {%s} (json): %s (%d)\n",
					img.Id, err, res.StatusCode)
			}
		}

		req2, err := http.NewRequest("PUT", REGISTRY_ENDPOINT+"/images/"+img.Id+"/layer", nil)
		req2.SetBasicAuth(authConfig.Username, authConfig.Password)
		res2, err := client.Do(req2)
		if err != nil || res2.StatusCode != 307 {
			return fmt.Errorf(
				"Error trying to push image {%s} (layer 1): %s\n",
				img.Id, err)
		}
		url, err := res2.Location()
		if err != nil || url == nil {
			return fmt.Errorf(
				"Fail to retrieve layer storage URL for image {%s}: %s\n",
				img.Id, err)
		}
		// FIXME: Don't do this :D. Check the S3 requierement and implement chunks of 5MB
		// FIXME2: I won't stress it enough, DON'T DO THIS! very high priority
		layerData2, err := Tar(path.Join(graph.Root, img.Id, "layer"), Gzip)
		layerData, err := Tar(path.Join(graph.Root, img.Id, "layer"), Gzip)
		if err != nil {
			return fmt.Errorf(
				"Error while retrieving layer for {%s}: %s\n",
				img.Id, err)
		}
		req3, err := http.NewRequest("PUT", url.String(), layerData)
		if err != nil {
			return err
		}
		tmp, err := ioutil.ReadAll(layerData2)
		if err != nil {
			return err
		}
		req3.ContentLength = int64(len(tmp))

		req3.TransferEncoding = []string{"none"}
		res3, err := client.Do(req3)
		if err != nil || res3.StatusCode != 200 {
			if res3 == nil {
				return fmt.Errorf(
					"Error trying to push image {%s} (layer 2): %s\n",
					img.Id, err)
			} else {
				return fmt.Errorf(
					"Error trying to push image {%s} (layer 2): %s (%d)\n",
					img.Id, err, res3.StatusCode)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (graph *Graph) pushTag(user, repo, revision, tag string, authConfig *auth.AuthConfig) error {

	if tag == "" {
		tag = "lastest"
	}

	revision = "\"" + revision + "\""

	client := &http.Client{}
	req, err := http.NewRequest("PUT", REGISTRY_ENDPOINT+"/users/"+user+"/"+repo+"/"+tag, strings.NewReader(revision))
	req.Header.Add("Content-type", "application/json")
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	fmt.Printf("Result of push tag: %d\n", res.StatusCode)
	switch res.StatusCode {
	default:
		return fmt.Errorf("Error %d\n", res.StatusCode)
	case 200:
	case 201:
	}
	return nil
}

func (graph *Graph) PushRepository(user, repoName string, repo Repository, authConfig *auth.AuthConfig) error {
	for tag, imgId := range repo {
		fmt.Printf("tag: %s, imgId: %s\n", tag, imgId)
		img, err := graph.Get(imgId)
		if err != nil {
			return err
		}
		if err = graph.PushImage(img, authConfig); err != nil {
			return err
		}
		if err = graph.pushTag(user, repoName, imgId, tag, authConfig); err != nil {
			return err
		}
	}
	return nil
}
