package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

//FIXME: Set the endpoint in a conf file or via commandline
//const INDEX_ENDPOINT = "http://registry-creack.dotcloud.com/v1"
const INDEX_ENDPOINT = auth.INDEX_SERVER + "/v1"

// Build an Image object from raw json data
func NewImgJson(src []byte) (*Image, error) {
	ret := &Image{}

	Debugf("Json string: {%s}\n", src)
	// FIXME: Is there a cleaner way to "purify" the input json?
	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// Build an Image object list from a raw json data
// FIXME: Do this in "stream" mode
func NewMultipleImgJson(src []byte) ([]*Image, error) {
	ret := []*Image{}

	dec := json.NewDecoder(strings.NewReader(string(src)))
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
func (graph *Graph) getRemoteHistory(imgId, registry string, token []string) ([]*Image, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/ancestry", nil)
	if err != nil {
		return nil, err
	}
	req.Header["X-Docker-Token"] = token
	// req.SetBasicAuth(authConfig.Username, authConfig.Password)
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
func (graph *Graph) LookupRemoteImage(imgId, registry string, authConfig *auth.AuthConfig) bool {
	rt := &http.Transport{Proxy: http.ProxyFromEnvironment}

	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/json", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := rt.RoundTrip(req)
	return err == nil && res.StatusCode == 307
}

// Retrieve an image from the Registry.
// Returns the Image object as well as the layer as an Archive (io.Reader)
func (graph *Graph) getRemoteImage(stdout io.Writer, imgId, registry string, token []string) (*Image, Archive, error) {
	client := &http.Client{}

	fmt.Fprintf(stdout, "Pulling %s metadata\r\n", imgId)
	// Get the Json
	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/json", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to download json: %s", err)
	}
	req.Header["X-Docker-Token"] = token
	// req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to download json: %s", err)
	}
	if res.StatusCode != 200 {
		return nil, nil, fmt.Errorf("HTTP code %d", res.StatusCode)
	}
	defer res.Body.Close()

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to download json: %s", err)
	}

	img, err := NewImgJson(jsonString)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse json: %s", err)
	}
	img.Id = imgId

	// Get the layer
	fmt.Fprintf(stdout, "Pulling %s fs layer\r\n", imgId)
	req, err = http.NewRequest("GET", registry+"/images/"+imgId+"/layer", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	req.Header["X-Docker-Token"] = token
	// req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err = client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	return img, ProgressReader(res.Body, int(res.ContentLength), stdout, "Downloading %v/%v (%v)"), nil
}

func (graph *Graph) PullImage(stdout io.Writer, imgId, registry string, token []string) error {
	history, err := graph.getRemoteHistory(imgId, registry, token)
	if err != nil {
		return err
	}
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines
	for _, j := range history {
		if !graph.Exists(j.Id) {
			img, layer, err := graph.getRemoteImage(stdout, j.Id, registry, token)
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

// // FIXME: Handle the askedTag parameter
// func (graph *Graph) PullRepository(stdout io.Writer, remote, askedTag, registry string, repositories *TagStore, authConfig *auth.AuthConfig) error {
// 	client := &http.Client{}

// 	fmt.Fprintf(stdout, "Pulling repository %s\r\n", remote)

// 	var repositoryTarget string
// 	// If we are asking for 'root' repository, lookup on the Library's registry
// 	if strings.Index(remote, "/") == -1 {
// 		repositoryTarget = registry + "/library/" + remote
// 	} else {
// 		repositoryTarget = registry + "/users/" + remote
// 	}

// 	req, err := http.NewRequest("GET", repositoryTarget, nil)
// 	if err != nil {
// 		return err
// 	}
// 	req.SetBasicAuth(authConfig.Username, authConfig.Password)
// 	res, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer res.Body.Close()
// 	if res.StatusCode != 200 {
// 		return fmt.Errorf("HTTP code: %d", res.StatusCode)
// 	}
// 	rawJson, err := ioutil.ReadAll(res.Body)
// 	if err != nil {
// 		return err
// 	}
// 	t := map[string]string{}
// 	if err = json.Unmarshal(rawJson, &t); err != nil {
// 		return err
// 	}
// 	for tag, rev := range t {
// 		fmt.Fprintf(stdout, "Pulling tag %s:%s\r\n", remote, tag)
// 		if err = graph.PullImage(stdout, rev, registry, authConfig); err != nil {
// 			return err
// 		}
// 		if err = repositories.Set(remote, tag, rev, true); err != nil {
// 			return err
// 		}
// 	}
// 	if err = repositories.Save(); err != nil {
// 		return err
// 	}
// 	return nil
// }

func (graph *Graph) PullRepository(stdout io.Writer, remote, askedTag string, repositories *TagStore, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	fmt.Fprintf(stdout, "Pulling repository %s\r\n", remote)
	var repositoryTarget string
	// If we are asking for 'root' repository, lookup on the Library's registry
	if strings.Index(remote, "/") == -1 {
		repositoryTarget = INDEX_ENDPOINT + "/repositories/library/" + remote + "/checksums"
	} else {
		repositoryTarget = INDEX_ENDPOINT + "/repositories/" + remote + "/checksums"
	}

	req, err := http.NewRequest("GET", repositoryTarget, nil)
	if err != nil {
		return err
	}
	if authConfig != nil {
		req.SetBasicAuth(authConfig.Username, authConfig.Password)
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// TODO: Right now we're ignoring checksums in the response body.
	// In the future, we need to use them to check image validity.
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP code: %d", res.StatusCode)
	}

	var token, endpoints []string
	if res.Header.Get("X-Docker-Token") != "" {
		token = res.Header["X-Docker-Token"]
	}
	if res.Header.Get("X-Docker-Endpoints") != "" {
		endpoints = res.Header["X-Docker-Endpoints"]
	} else {
		return fmt.Errorf("Index response didn't contain any endpoints")
	}

	// FIXME: If askedTag is empty, fetch all tags.
	if askedTag == "" {
		askedTag = "latest"
	}

	for _, registry := range endpoints {
		registryEndpoint := "https://" + registry + "/v1"
		if strings.Index(remote, "/") == -1 {
			repositoryTarget = registryEndpoint + "/repositories/library/" +
				remote + "/tags/" + askedTag
		} else {
			repositoryTarget = registryEndpoint + "/repositories/users/" +
				remote + "/tags/" + askedTag
		}

		req, err = http.NewRequest("GET", repositoryTarget, nil)
		if err != nil {
			return err
		}
		req.Header["X-Docker-Token"] = token
		res, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(stdout, "Error while retrieving repository info: %v ; " +
				"checking next endpoint")
			continue
		}
		defer res.Body.Close()
		if res.StatusCode == 403 {
			if authConfig == nil {
				return fmt.Errorf("You need to be authenticated to access this resource")
			} else {
				return fmt.Errorf("You aren't authorized to access this resource")
			}
		} else if res.StatusCode != 200 {
			return fmt.Errorf("HTTP code: %d", res.StatusCode)
		}

		var imgId string
		rawJson, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(rawJson, &imgId); err != nil {
			return err
		}

		if err := graph.PullImage(stdout, imgId, registryEndpoint, token); err != nil {
			return err
		}

		if err = repositories.Set(remote, askedTag, imgId, true); err != nil {
			return err
		}

		if err = repositories.Save(); err != nil {
			return err
		}

		return nil
	}
	return fmt.Errorf("Could not find repository on any of the indexed registries.")
}

// Push a local image to the registry with its history if needed
func (graph *Graph) PushImage(stdout io.Writer, imgOrig *Image, registry string, authConfig *auth.AuthConfig) error {
	client := &http.Client{}

	// FIXME: Factorize the code
	// FIXME: Do the puts in goroutines
	if err := imgOrig.WalkHistory(func(img *Image) error {

		jsonRaw, err := ioutil.ReadFile(path.Join(graph.Root, img.Id, "json"))
		if err != nil {
			return fmt.Errorf("Error while retreiving the path for {%s}: %s", img.Id, err)
		}

		fmt.Fprintf(stdout, "Pushing %s metadata\r\n", img.Id)

		// FIXME: try json with UTF8
		jsonData := strings.NewReader(string(jsonRaw))
		req, err := http.NewRequest("PUT", registry+"/images/"+img.Id+"/json", jsonData)
		if err != nil {
			return err
		}
		req.Header.Add("Content-type", "application/json")
		req.SetBasicAuth(authConfig.Username, authConfig.Password)
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("Failed to upload metadata: %s", err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			switch res.StatusCode {
			case 204:
				// Case where the image is already on the Registry
				// FIXME: Do not be silent?
				return nil
			default:
				errBody, err := ioutil.ReadAll(res.Body)
				if err != nil {
					errBody = []byte(err.Error())
				}
				return fmt.Errorf("HTTP code %d while uploading metadata: %s", res.StatusCode, errBody)
			}
		}

		fmt.Fprintf(stdout, "Pushing %s fs layer\r\n", img.Id)
		req2, err := http.NewRequest("PUT", registry+"/images/"+img.Id+"/layer", nil)
		req2.SetBasicAuth(authConfig.Username, authConfig.Password)
		res2, err := client.Do(req2)
		if err != nil {
			return fmt.Errorf("Registry returned error: %s", err)
		}
		res2.Body.Close()
		if res2.StatusCode != 307 {
			return fmt.Errorf("Registry returned unexpected HTTP status code %d, expected 307", res2.StatusCode)
		}
		url, err := res2.Location()
		if err != nil || url == nil {
			return fmt.Errorf("Failed to retrieve layer upload location: %s", err)
		}

		// FIXME: stream the archive directly to the registry instead of buffering it on disk. This requires either:
		//	a) Implementing S3's proprietary streaming logic, or
		//	b) Stream directly to the registry instead of S3.
		// I prefer option b. because it doesn't lock us into a proprietary cloud service.
		tmpLayer, err := graph.TempLayerArchive(img.Id, Xz, stdout)
		if err != nil {
			return err
		}
		defer os.Remove(tmpLayer.Name())
		req3, err := http.NewRequest("PUT", url.String(), ProgressReader(tmpLayer, int(tmpLayer.Size), stdout, "Uploading %v/%v (%v)"))
		if err != nil {
			return err
		}
		req3.ContentLength = int64(tmpLayer.Size)

		req3.TransferEncoding = []string{"none"}
		res3, err := client.Do(req3)
		if err != nil {
			return fmt.Errorf("Failed to upload layer: %s", err)
		}
		res3.Body.Close()
		if res3.StatusCode != 200 {
			return fmt.Errorf("Received HTTP code %d while uploading layer", res3.StatusCode)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// push a tag on the registry.
// Remote has the format '<user>/<repo>
func (graph *Graph) pushTag(remote, revision, tag, registry string, authConfig *auth.AuthConfig) error {

	// Keep this for backward compatibility
	if tag == "" {
		tag = "lastest"
	}

	// "jsonify" the string
	revision = "\"" + revision + "\""

	Debugf("Pushing tags for rev [%s] on {%s}\n", revision, registry+"/users/"+remote+"/"+tag)

	client := &http.Client{}
	req, err := http.NewRequest("PUT", registry+"/users/"+remote+"/"+tag, strings.NewReader(revision))
	req.Header.Add("Content-type", "application/json")
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return fmt.Errorf("Internal server error: %d trying to push tag %s on %s", res.StatusCode, tag, remote)
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

func (graph *Graph) LookupRemoteRepository(remote, registry string, authConfig *auth.AuthConfig) bool {
	rt := &http.Transport{Proxy: http.ProxyFromEnvironment}

	var repositoryTarget string
	// If we are asking for 'root' repository, lookup on the Library's registry
	if strings.Index(remote, "/") == -1 {
		repositoryTarget = registry + "/library/" + remote + "/lookup"
	} else {
		parts := strings.Split(remote, "/")
		if len(parts) != 2 {
			Debugf("Repository must abide to following format: user/repo_name")
			return false
		}
		repositoryTarget = registry + "/users/" + parts[0] + "/repositories/" + parts[1]
	}
	Debugf("Checking for permissions on: %s", repositoryTarget)
	req, err := http.NewRequest("PUT", repositoryTarget, strings.NewReader("\"\""))
	if err != nil {
		Debugf("%s\n", err)
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	req.Header.Add("Content-type", "application/json")
	res, err := rt.RoundTrip(req)
	if err != nil || res.StatusCode != 404 {
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			errBody = []byte(err.Error())
		}
		Debugf("Lookup status code: %d (body: %s)", res.StatusCode, errBody)
		return false
	}
	return true
}

// FIXME: this should really be PushTag
func (graph *Graph) pushPrimitive(stdout io.Writer, remote, tag, imgId, registry string, authConfig *auth.AuthConfig) error {
	// Check if the local impage exists
	img, err := graph.Get(imgId)
	if err != nil {
		fmt.Fprintf(stdout, "Skipping tag %s:%s: %s does not exist\r\n", remote, tag, imgId)
		return nil
	}
	fmt.Fprintf(stdout, "Pushing tag %s:%s\r\n", remote, tag)
	// Push the image
	if err = graph.PushImage(stdout, img, registry, authConfig); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Registering tag %s:%s\r\n", remote, tag)
	// And then the tag
	if err = graph.pushTag(remote, imgId, registry, tag, authConfig); err != nil {
		return err
	}
	return nil
}

// Push a repository to the registry.
// Remote has the format '<user>/<repo>
func (graph *Graph) PushRepository(stdout io.Writer, remote, registry string, localRepo Repository, authConfig *auth.AuthConfig) error {
	// Check if the remote repository exists/if we have the permission
	if !graph.LookupRemoteRepository(remote, registry, authConfig) {
		return fmt.Errorf("Permission denied on repository %s\n", remote)
	}

	fmt.Fprintf(stdout, "Pushing repository %s (%d tags)\r\n", remote, len(localRepo))
	// For each image within the repo, push them
	for tag, imgId := range localRepo {
		if err := graph.pushPrimitive(stdout, remote, tag, imgId, registry, authConfig); err != nil {
			// FIXME: Continue on error?
			return err
		}
	}
	return nil
}
