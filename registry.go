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

	Debugf("Json string: {%s}\n", src)
	// FIXME: Is there a cleaner way to "puryfy" the input json?
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
	if err != nil || res.StatusCode != 200 {
		if res != nil {
			return nil, fmt.Errorf("Internal server error: %d trying to fetch remote history for %s", res.StatusCode, imgId)
		}
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
	rt := &http.Transport{Proxy: http.ProxyFromEnvironment}

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/images/"+imgId+"/json", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := rt.RoundTrip(req)
	if err != nil || res.StatusCode != 307 {
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
	if err != nil || res.StatusCode != 200 {
		if res != nil {
			return nil, nil, fmt.Errorf("Internal server error: %d trying to get image %s", res.StatusCode, imgId)
		}
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
	req, err = http.NewRequest("GET", REGISTRY_ENDPOINT+"/images/"+imgId+"/layer", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err = client.Do(req)
	if err != nil {
		return nil, nil, err
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
func (graph *Graph) PullRepository(stdout io.Writer, remote, askedTag string, repositories *TagStore, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	fmt.Fprintf(stdout, "Pulling repo: %s\n", REGISTRY_ENDPOINT+"/users/"+remote)

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/users/"+remote, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil || res.StatusCode != 200 {
		if res != nil {
			return fmt.Errorf("Internal server error: %d trying to pull %s", res.StatusCode, remote)
		}
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
		if err = repositories.Set(remote, tag, rev, true); err != nil {
			return err
		}
	}
	if err = repositories.Save(); err != nil {
		return err
	}
	return nil
}

// Push a local image to the registry with its history if needed
func (graph *Graph) PushImage(stdout io.Writer, imgOrig *Image, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	// FIXME: Factorize the code
	// FIXME: Do the puts in goroutines
	if err := imgOrig.WalkHistory(func(img *Image) error {

		jsonRaw, err := ioutil.ReadFile(path.Join(graph.Root, img.Id, "json"))
		if err != nil {
			return fmt.Errorf("Error while retreiving the path for {%s}: %s", img.Id, err)
		}

		fmt.Fprintf(stdout, "Pushing image [%s] on {%s}\n", img.Id, REGISTRY_ENDPOINT+"/images/"+img.Id+"/json")

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
			Debugf("Pushing return status: %d\n", res.StatusCode)
			switch res.StatusCode {
			case 204:
				// Case where the image is already on the Registry
				// FIXME: Do not be silent?
				fmt.Fprintf(stdout, "The image %s is already up to date on the registry.\n", img.Id)
				return nil
			case 400:
				return fmt.Errorf("Error: Invalid Json")
			default:
				return fmt.Errorf(
					"Error: Internal server error: %d trying to push image {%s} (json): %s\n",
					res.StatusCode, img.Id, err)
			}
		}

		req2, err := http.NewRequest("PUT", REGISTRY_ENDPOINT+"/images/"+img.Id+"/layer", nil)
		req2.SetBasicAuth(authConfig.Username, authConfig.Password)
		res2, err := client.Do(req2)
		if err != nil || res2.StatusCode != 307 {
			return fmt.Errorf(
				"Internal server error trying to push image {%s} (layer 1): %s\n",
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
			}
			return fmt.Errorf(
				"Error trying to push image {%s} (layer 2): %s (%d)\n",
				img.Id, err, res3.StatusCode)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// push a tag on the registry.
// Remote has the format '<user>/<repo>
func (graph *Graph) pushTag(remote, revision, tag string, authConfig *auth.AuthConfig) error {

	// Keep this for backward compatibility
	if tag == "" {
		tag = "lastest"
	}

	// "jsonify" the string
	revision = "\"" + revision + "\""

	Debugf("Pushing tags for rev [%s] on {%s}\n", revision, REGISTRY_ENDPOINT+"/users/"+remote+"/"+tag)

	client := &http.Client{}
	req, err := http.NewRequest("PUT", REGISTRY_ENDPOINT+"/users/"+remote+"/"+tag, strings.NewReader(revision))
	req.Header.Add("Content-type", "application/json")
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil || (res.StatusCode != 200 && res.StatusCode != 201) {
		if res != nil {
			return fmt.Errorf("Internal server error: %d trying to push tag %s on %s", res.StatusCode, tag, remote)
		}
		return err
	}
	Debugf("Result of push tag: %d\n", res.StatusCode)
	switch res.StatusCode {
	default:
		return fmt.Errorf("Error %d\n", res.StatusCode)
	case 200:
	case 201:
	}
	return nil
}

func (graph *Graph) LookupRemoteRepository(remote string, authConfig *auth.AuthConfig) bool {
	rt := &http.Transport{Proxy: http.ProxyFromEnvironment}

	req, err := http.NewRequest("GET", REGISTRY_ENDPOINT+"/users/"+remote, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := rt.RoundTrip(req)
	if err != nil || res.StatusCode != 200 {
		return false
	}
	return true
}

func (graph *Graph) pushPrimitive(stdout io.Writer, remote, tag, imgId string, authConfig *auth.AuthConfig) error {
	// CHeck if the local impage exists
	img, err := graph.Get(imgId)
	if err != nil {
		return err
	}
	// Push the image
	if err = graph.PushImage(stdout, img, authConfig); err != nil {
		return err
	}
	// And then the tag
	if err = graph.pushTag(remote, imgId, tag, authConfig); err != nil {
		return err
	}
	return nil
}

// Push a repository to the registry.
// Remote has the format '<user>/<repo>
func (graph *Graph) PushRepository(stdout io.Writer, remote string, localRepo Repository, authConfig *auth.AuthConfig) error {
	// Check if the remote repository exists
	// FIXME: @lopter How to handle this?
	// if !graph.LookupRemoteRepository(remote, authConfig) {
	// 	return fmt.Errorf("The remote repository %s does not exist\n", remote)
	// }

	// For each image within the repo, push them
	for tag, imgId := range localRepo {
		if err := graph.pushPrimitive(stdout, remote, tag, imgId, authConfig); err != nil {
			// FIXME: Continue on error?
			return err
		}
	}
	return nil
}
