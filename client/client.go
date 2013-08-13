package client

import (
	"archive/tar"
    "bytes"
    "encoding/json"
    "fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/term"
    "io"
    "io/ioutil"
    "net"
    "net/http"
    "net/http/httputil"
    "net/url"
    "os"
    "strconv"
    "strings"
)

type JSONReader func(*utils.JSONMessage) error

func NewClient(proto, addr string) *Client {
    configFile, err := auth.LoadConfig(os.Getenv("HOME"))

	if err != nil {
		fmt.Printf("WARNING: %s\n", err)
	}

    return &Client{
        addr: addr,
        protocol: proto,
        configFile: configFile,
    }
}

func (c *Client) Addr() string {
    return c.addr
}

func (c *Client) Protocol() string {
    return c.protocol
}

func (c *Client) Config(i string) (auth.AuthConfig, bool) {
    conf, ok := c.configFile.Configs[i]
    return conf, ok
}

type Client struct {
    protocol string
    addr string
	configFile *auth.ConfigFile
}

func (c *Client) Build(dockerfile, tag string, quiet, noCache bool, out io.Writer, progressOut io.Writer) error {
	var (
		context  utils.Archive
		isRemote bool
		err      error
	)

	if utils.IsURL(dockerfile) || utils.IsGIT(dockerfile) {
		isRemote = true
	} else if _, err := os.Stat(dockerfile); err == nil {
		context, err = utils.Tar(dockerfile, utils.Uncompressed)
	} else {
        context, err = mkBuildContext(dockerfile, nil)
    }

    if err != nil {
        return err
    }

	var body io.Reader

	if context != nil {
		sf := utils.NewStreamFormatter(false)
		body = utils.ProgressReader(ioutil.NopCloser(context), 0, progressOut, sf.FormatProgress("", "Uploading context", "%v bytes%0.0s%0.0s"), sf, true)
	}

	// Upload the build context
	v := &url.Values{}
	v.Set("t", tag)

	if quiet {
		v.Set("q", "1")
	}
	if isRemote {
		v.Set("remote", dockerfile)
	}
	if noCache {
		v.Set("nocache", "1")
	}
	req, err := c.newRequest("POST", fmt.Sprintf("/build?%s", v.Encode()), body)
	if err != nil {
		return err
	}
	if context != nil {
		req.Header.Set("Content-Type", "application/tar")
	}
	dial, err := net.Dial(c.protocol, c.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(body) == 0 {
            return NewAPIError(resp.StatusCode, "Error: %s", http.StatusText(resp.StatusCode))
		}
        return NewAPIError(resp.StatusCode, "Error: %s", body)
	}

	// Output the result
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	return nil
}

func (c *Client) Events(since string, reader JSONReader) error {
	v := url.Values{}
	if since != "" {
		v.Set("since", since)
	}

	return c.streamJSON("GET", "/events?"+v.Encode(), nil, reader)
}

func (c *Client) Authenticate(username, password, email string) (string, error) {
	authconfig, ok := c.configFile.Configs[auth.IndexServerAddress()]
	if !ok {
		authconfig = auth.AuthConfig{}
	}

    if username != "" {
        authconfig.Username = username
    }
    if password != "" {
        authconfig.Password = password
    }
    if email != "" {
        authconfig.Email = email
    }
	c.configFile.Configs[auth.IndexServerAddress()] = authconfig

	body, statusCode, err := c.call("POST", "/auth", c.configFile.Configs[auth.IndexServerAddress()])

	if statusCode == 401 {
		delete(c.configFile.Configs, auth.IndexServerAddress())
		auth.SaveConfig(c.configFile)
		return "", err
	}
	if err != nil {
		return "", err
	}

	var out2 docker.APIAuth
	err = json.Unmarshal(body, &out2)
	if err != nil {
		c.configFile, _ = auth.LoadConfig(os.Getenv("HOME"))
		return "", err
	}
	auth.SaveConfig(c.configFile)
    return out2.Status, nil
}

func (c *Client) Info() (*docker.APIInfo, error) {
	body, _, err := c.call("GET", "/info", nil)
	if err != nil {
		return nil, err
	}

	var out docker.APIInfo
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

    return &out, nil
}

func (c *Client) Version() (*docker.APIVersion, error) {
	body, _, err := c.call("GET", "/version", nil)
	if err != nil {
		return nil, err
	}

	var out docker.APIVersion
	if err = json.Unmarshal(body, &out); err != nil {
        return nil, err
    }

    return &out, nil
}

func (c *Client) Commit(container, repo, tag, comment, author string, config *docker.Config) (string, error){
	v := url.Values{}
	v.Set("container", container)
	v.Set("repo", repo)
	v.Set("tag", tag)
	v.Set("comment", comment)
	v.Set("author", author)

	body, _, err := c.call("POST", "/commit?"+v.Encode(), config)
	if err != nil {
		return "", err
	}

	id := &docker.APIID{}
	err = json.Unmarshal(body, id)
	if err != nil {
		return "", err
	}
    return id.ID, nil
}

func (c *Client) ContainerList(size bool, all bool, limit int, since string, before string) ([]docker.APIContainers, error) {
	v := url.Values{}

	if all {
		v.Set("all", "1")
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if since != "" {
		v.Set("since", since)
	}
	if before != "" {
		v.Set("before", before)
	}
	if size {
		v.Set("size", "1")
	}

	body, _, err := c.call("GET", "/containers/json?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var outs []docker.APIContainers
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return nil, err
	}
	return outs, nil
}

func (c *Client) ContainerCreate(config *docker.Config) (*docker.APIRun, error) {
	//create the container
	body, _, err := c.call("POST", "/containers/create", config)
    if err != nil {
        return nil, err
    }

	runResult := &docker.APIRun{}
	err = json.Unmarshal(body, runResult)
	if err != nil {
		return nil, err
	}

    return runResult, err
}

func (c *Client) ContainerInspect(cid string) (*docker.Container, error) {
    body, _, err := c.call("GET", "/containers/"+cid+"/json", nil)
    if err != nil {
        return nil, err
    }

	container := &docker.Container{}
	err = json.Unmarshal(body, container)
	if err != nil {
		return nil, err
	}

    return container, nil
}

func (c *Client) ContainerTop(cid string, args ...string) (*docker.APITop, error) {
	val := url.Values{}
	if len(args) > 1 {
		val.Set("ps_args", strings.Join(args, " "))
	}

	body, _, err := c.call("GET", "/containers/"+cid+"/top?"+val.Encode(), nil)
	if err != nil {
		return nil, err
	}
	procs := docker.APITop{}
	err = json.Unmarshal(body, &procs)
	if err != nil {
		return nil, err
	}
    return &procs, nil
}

func (c *Client) ContainerDiff(cid string) ([]docker.Change, error) {
	body, _, err := c.call("GET", "/containers/"+cid+"/changes", nil)
	if err != nil {
		return nil, err
	}

	changes := []docker.Change{}
	err = json.Unmarshal(body, &changes)
	if err != nil {
		return nil, err
	}

    return changes, nil
}

func (c *Client) ContainerExport(cid string, out io.Writer) error {
	return c.stream("GET", "/containers/"+cid+"/export", nil, out)
}

func (c *Client) ContainerStart(cid string, config *docker.HostConfig) error {
    _, _, err := c.call("POST", "/containers/"+cid+"/start", config)
    return err
}

func (c *Client) ContainerStop(cid string, wait int) error {
	v := url.Values{}
	v.Set("t", strconv.Itoa(wait))
    _, _, err := c.call("POST", "/containers/"+cid+"/stop?"+v.Encode(), nil)

    return err
}

func (c *Client) ContainerRestart(cid string, wait int) error {
	v := url.Values{}
	v.Set("t", strconv.Itoa(wait))
    _, _, err := c.call("POST", "/containers/"+cid+"/restart?"+v.Encode(), nil)
    return err
}

func (c *Client) ContainerKill(cid string) error {
    _, _, err := c.call("POST", "/containers/"+cid+"/kill", nil)
    return err
}

func (c *Client) ContainerAttach(cid string, logs, stream, stdin, stdout, stderr bool, in io.ReadCloser, out io.Writer, terminalFd *uintptr) error {
    container, err := c.ContainerInspect(cid)

	if err != nil {
		return err
	}

	if !container.State.Running {
		return fmt.Errorf("Impossible to attach to a stopped container, start it first")
	}

	v := url.Values{}
    if logs {
        v.Set("logs", "1")
    }
    if stream {
        v.Set("stream", "1")
    }
    if stdin {
        v.Set("stdin", "1")
    }
    if stdout {
        v.Set("stdout", "1")
    }
    if stderr {
        v.Set("stderr", "1")
    }

	if err := c.hijack("POST", "/containers/"+cid+"/attach?"+v.Encode(), in, out, terminalFd); err != nil {
		return err
	}
	return nil
}

func (c *Client) ContainerWait(cid string) (int, error) {
    body, _, err := c.call("POST", "/containers/"+cid+"/wait", nil)
    if err != nil {
        return -1, err
    }

    var out docker.APIWait
    err = json.Unmarshal(body, &out)
    if err != nil {
        return -1, err
    }

    return out.StatusCode, nil
}

func (c *Client) ContainerRemove(cid string, volumes bool) error {
	val := url.Values{}
	if volumes {
		val.Set("v", "1")
	}

    _, _, err := c.call("DELETE", "/containers/"+cid+"?"+val.Encode(), nil)
    return err
}

func (c *Client) ContainerCopy(cid, resource string) ([]byte, error) {
    copyData := docker.APICopy{
        Resource: resource,
    }

	data, _, err := c.call("POST", "/containers/"+cid+"/copy", copyData)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) ImageList(all bool, filter string) ([]docker.APIImages, error) {
    v := url.Values{}
    if filter != "" {
        v.Set("filter", filter)
    }
    if all {
        v.Set("all", "1")
    }
    body, _, err := c.call("GET", "/images/json?"+v.Encode(), nil)
    if err != nil {
        return nil, err
    }

    var outs []docker.APIImages
    err = json.Unmarshal(body, &outs)
    if err != nil {
        return nil, err
    }

    return outs, nil
}

func (c *Client) ImageListViz() (string, error) {
    body, _, err := c.call("GET", "/images/viz", false)
    if err != nil {
        return "", err
    }

    return string(body), nil
}

func (c *Client) ImageCreate(image, src, repo, tag, registry string, reader JSONReader, in io.Reader) error {
	v := url.Values{}

    if (image != "") {
        v.Set("fromImage", image)
    }
    if (src != "") {
        v.Set("fromSrc", src)
    }
    if (repo != "") {
        v.Set("repo", repo)
    }
    if (tag != "") {
        v.Set("tag", repo)
    }
    if (registry != "") {
        v.Set("registry", registry)
    }

	return c.streamJSON("POST", "/images/create?"+v.Encode(), in, reader)
}

func (c *Client) ImageInsert(name, u, path string, reader JSONReader) error {
	v := url.Values{}
	v.Set("url", u)
	v.Set("path", path)

	return c.streamJSON("POST", "/images/"+name+"/insert?"+v.Encode(), nil, reader)
}

func (c *Client) ImageInspect(name string) (*docker.Image, error) {
    body, _, err := c.call("GET", "/images/"+name+"/json", nil)
    if err != nil {
        return nil, err
    }

	img := &docker.Image{}
	err = json.Unmarshal(body, img)
	if err != nil {
		return nil, err
	}

    return img, nil
}

func (c *Client) ImageHistory(name string) ([]docker.APIHistory, error) {
	body, _, err := c.call("GET", "/images/"+name+"/history", nil)
	if err != nil {
		return nil, err
	}
	var outs []docker.APIHistory
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return nil, err
	}
	for _, out := range outs {
		if out.Tags != nil {
			out.ID = out.Tags[0]
		}
	}

    return outs, nil
}

func (c *Client) ImagePush(name string, reader JSONReader) error {
	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if len(strings.SplitN(name, "/", 2)) == 1 {
		return fmt.Errorf("Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)", c.configFile.Configs[auth.IndexServerAddress()].Username, name)
	}

	v := url.Values{}
	push := func() error {
		buf, err := json.Marshal(c.configFile.Configs[auth.IndexServerAddress()])
		if err != nil {
			return err
		}

		return c.streamJSON("POST", "/images/"+name+"/push?"+v.Encode(), bytes.NewBuffer(buf), reader)
	}

	if err := push(); err != nil {
		if err.Error() == "Authentication is required." {
			if _, err := c.Authenticate("", "", ""); err != nil {
				return err
			}
            return push()
		}
		return err
	}
	return nil
}

func (c *Client) ImageTag(name, repo, tag string, force bool) error {
	v := url.Values{}
	v.Set("repo", repo)
	if tag != "" {
		v.Set("tag", tag)
	}

	if force {
		v.Set("force", "1")
	}

	_, _, err := c.call("POST", "/images/"+name+"/tag?"+v.Encode(), nil)
    return err
}

func (c *Client) ImageRemove(name string) ([]docker.APIRmi, error) {
    body, _, err := c.call("DELETE", "/images/"+name, nil)
    var outs []docker.APIRmi
    err = json.Unmarshal(body, &outs)
    if err != nil {
        return nil, err
    }

    return outs, nil
}

func (c *Client) ImageSearch(term string) ([]docker.APISearch, error) {
	v := url.Values{}
	v.Set("term", term)
	body, _, err := c.call("GET", "/images/search?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}

	outs := []docker.APISearch{}
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return nil, err
	}

    return outs, nil
}

func (c *Client) ContainerResize(cid string, width, height int) error {
	v := url.Values{}
	v.Set("h", strconv.Itoa(height))
	v.Set("w", strconv.Itoa(width))

	_, _, err := c.call("POST", "/containers/"+cid+"/resize?"+v.Encode(), nil)
    return err
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
    path = fmt.Sprintf("/v%g%s", docker.APIVERSION, path)
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	req.Host = c.addr

    utils.Debugf("[client] %s %s", method, path)

    return req, nil
}

func (c *Client) call(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}

	req, err := c.newRequest(method, path, params)
	if err != nil {
		return nil, -1, err
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(c.protocol, c.addr)
	if err != nil {
		return nil, -1, err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error: %s", http.StatusText(resp.StatusCode))
		}
		return nil, resp.StatusCode, fmt.Errorf("Error: %s", body)
	}
	return body, resp.StatusCode, nil
}

func (c *Client) streamJSON(method, path string, in io.Reader, jsonReader JSONReader) error {
    r, w := io.Pipe()

    go func() {
        dec := json.NewDecoder(r)

        for {
            mes := &utils.JSONMessage{}

            if err := dec.Decode(mes); err == io.EOF {
                break
            } else if err != nil {
                mes.ErrorMessage = err.Error()
            }

            jsonReader(mes)
        }
    }()

    return c.stream(method, path, in, w)
}

func (c *Client) stream(method, path string, in io.Reader, out io.Writer) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := c.newRequest(method, path, in)
	if err != nil {
		return err
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(c.protocol, c.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("Error :%s", http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("Error: %s", body)
	}

    if _, err := io.Copy(out, resp.Body); err != nil {
        return err
    }
	return nil
}

func (c *Client) hijack(method, path string, in io.ReadCloser, out io.Writer, terminalFd *uintptr) error {
	req, err := c.newRequest(method, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "plain/text")

	dial, err := net.Dial(c.protocol, c.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	clientconn.Do(req)

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	receiveStdout := utils.Go(func() error {
		_, err := io.Copy(out, br)
		utils.Debugf("[hijack] End of stdout")
		return err
	})

	if in != nil && terminalFd != nil {
		oldState, err := term.SetRawTerminal(*terminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(*terminalFd, oldState)
	}

	sendStdin := utils.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			utils.Debugf("[hijack] End of stdin")
		}
		if tcpc, ok := rwc.(*net.TCPConn); ok {
			if err := tcpc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		} else if unixc, ok := rwc.(*net.UnixConn); ok {
			if err := unixc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if err := <-receiveStdout; err != nil {
		utils.Debugf("Error receiveStdout: %s", err)
		return err
	}

    if terminalFd == nil {
        if err := <-sendStdin; err != nil {
            utils.Debugf("Error sendStdin: %s", err)
            return err
        }
    }

	return nil
}

// mkBuildContext returns an archive of an empty context with the contents
// of `dockerfile` at the path ./Dockerfile
func mkBuildContext(dockerfile string, files [][2]string) (utils.Archive, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	files = append(files, [2]string{"Dockerfile", dockerfile})
	for _, file := range files {
		name, content := file[0], file[1]
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func NewAPIError(statusCode int, format string, a ...interface{}) *APIError {
    return &APIError{
        error: fmt.Sprintf(format, a...),
        StatusCode: statusCode,
    }
}

type APIError struct {
    error string
    StatusCode int
}

func (e *APIError) Error() string {
    return e.error
}
