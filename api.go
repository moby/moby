package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/gorilla/mux"
	"github.com/shin-/cookiejar"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func hijackServer(w http.ResponseWriter) (io.ReadCloser, io.Writer, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Flush the options to make sure the client sets the raw mode
	conn.Write([]byte{})
	return conn, conn, nil
}

//If we don't do this, POST method without Content-type (even with empty body) will fail
func parseForm(r *http.Request) error {
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func httpError(w http.ResponseWriter, err error) {
	if strings.HasPrefix(err.Error(), "No such") {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getAuth(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var out auth.AuthConfig
	out.Username = srv.runtime.authConfig.Username
	out.Email = srv.runtime.authConfig.Email
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func postAuth(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var config auth.AuthConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		return nil, err
	}

	if config.Username == srv.runtime.authConfig.Username {
		config.Password = srv.runtime.authConfig.Password
	}

	newAuthConfig := auth.NewAuthConfig(config.Username, config.Password, config.Email, srv.runtime.root)
	status, err := auth.Login(newAuthConfig)
	if err != nil {
		return nil, err
	} else {
		srv.runtime.graph.getHttpClient().Jar = cookiejar.NewCookieJar()
		srv.runtime.authConfig = newAuthConfig
	}
	if status != "" {
		b, err := json.Marshal(ApiAuth{status})
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func getVersion(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	m := srv.DockerVersion()
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func postContainersKill(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerKill(name); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func getContainersExport(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]

	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ContainerExport(name, out); err != nil {
		fmt.Fprintf(out, "Error: %s\n", err)
	}
	return nil, nil
}

func getImages(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}

	viz := r.Form.Get("viz") == "1"
	if viz {
		in, out, err := hijackServer(w)
		if err != nil {
			return nil, err
		}
		defer in.Close()
		fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
		if err := srv.ImagesViz(out); err != nil {
			fmt.Fprintf(out, "Error: %s\n", err)
		}
		return nil, nil
	}

	all := r.Form.Get("all") == "1"
	filter := r.Form.Get("filter")
	only_ids := r.Form.Get("only_ids") == "1"

	outs, err := srv.Images(all, only_ids, filter)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getInfo(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	out := srv.DockerInfo()
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getImagesHistory(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	outs, err := srv.ImageHistory(name)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getContainersChanges(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	changesStr, err := srv.ContainerChanges(name)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(changesStr)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getContainers(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	all := r.Form.Get("all") == "1"
	trunc_cmd := r.Form.Get("trunc_cmd") != "0"
	only_ids := r.Form.Get("only_ids") == "1"
	since := r.Form.Get("since")
	before := r.Form.Get("before")
	n, err := strconv.Atoi(r.Form.Get("limit"))
	if err != nil {
		n = -1
	}

	outs := srv.Containers(all, trunc_cmd, only_ids, n, since, before)
	b, err := json.Marshal(outs)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func postImagesTag(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	vars := mux.Vars(r)
	name := vars["name"]
	force := r.Form.Get("force") == "1"

	if err := srv.ContainerTag(name, repo, tag, force); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusCreated)
	return nil, nil
}

func postCommit(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		Debugf("%s", err.Error())
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	container := r.Form.Get("container")
	author := r.Form.Get("author")
	comment := r.Form.Get("comment")
	id, err := srv.ContainerCommit(container, repo, tag, author, comment, &config)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(ApiId{id})
	if err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusCreated)
	return b, nil
}

func postImages(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}

	src := r.Form.Get("fromSrc")
	image := r.Form.Get("fromImage")
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")

	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if image != "" { //pull
		registry := r.Form.Get("registry")
		if err := srv.ImagePull(image, tag, registry, out); err != nil {
			fmt.Fprintf(out, "Error: %s\n", err)
		}
	} else { //import
		if err := srv.ImageImport(src, repo, tag, in, out); err != nil {
			fmt.Fprintf(out, "Error: %s\n", err)
		}
	}
	return nil, nil
}

func getImagesSearch(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}

	term := r.Form.Get("term")
	outs, err := srv.ImagesSearch(term)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func postImagesInsert(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}

	url := r.Form.Get("url")
	path := r.Form.Get("path")
	vars := mux.Vars(r)
	name := vars["name"]

	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ImageInsert(name, url, path, out); err != nil {
		fmt.Fprintf(out, "Error: %s\n", err)
	}
	return nil, nil
}

func postImagesPush(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}

	registry := r.Form.Get("registry")

	vars := mux.Vars(r)
	name := vars["name"]

	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ImagePush(name, registry, out); err != nil {
		fmt.Fprintln(out, "Error: %s\n", err)
	}
	return nil, nil
}

func postBuild(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ImageCreateFromFile(in, out); err != nil {
		fmt.Fprintln(out, "Error: %s\n", err)
	}
	return nil, nil
}

func postContainers(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		return nil, err
	}
	id, memoryW, swapW, err := srv.ContainerCreate(config)
	if err != nil {
		return nil, err
	}
	var out ApiRun
	out.Id = id
	if memoryW {
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if swapW {
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusCreated)
	return b, nil
}

func postContainersRestart(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerRestart(name, t); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func deleteContainers(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	vars := mux.Vars(r)
	name := vars["name"]
	v := r.Form.Get("v") == "1"

	if err := srv.ContainerDestroy(name, v); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func deleteImages(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ImageDelete(name); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func postContainersStart(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerStart(name); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func postContainersStop(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}
	vars := mux.Vars(r)
	name := vars["name"]

	if err := srv.ContainerStop(name, t); err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil, nil
}

func postContainersWait(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	status, err := srv.ContainerWait(name)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(ApiWait{status})
	if err != nil {
		return nil, err
	}
	return b, nil
}

func postContainersAttach(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := parseForm(r); err != nil {
		return nil, err
	}
	logs := r.Form.Get("logs") == "1"
	stream := r.Form.Get("stream") == "1"
	stdin := r.Form.Get("stdin") == "1"
	stdout := r.Form.Get("stdout") == "1"
	stderr := r.Form.Get("stderr") == "1"
	vars := mux.Vars(r)
	name := vars["name"]

	in, out, err := hijackServer(w)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, in, out); err != nil {
		fmt.Fprintf(out, "Error: %s\n", err)
	}
	return nil, nil
}

func getContainersByName(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]

	container, err := srv.ContainerInspect(name)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(container)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getImagesByName(srv *Server, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	vars := mux.Vars(r)
	name := vars["name"]

	image, err := srv.ImageInspect(name)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(image)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ListenAndServe(addr string, srv *Server) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	m := map[string]map[string]func(*Server, http.ResponseWriter, *http.Request) ([]byte, error){
		"GET": {
			"/auth":                         getAuth,
			"/version":                      getVersion,
			"/containers/{name:.*}/export":  getContainersExport,
			"/images":                       getImages,
			"/info":                         getInfo,
			"/images/search":                getImagesSearch,
			"/images/{name:.*}/history":     getImagesHistory,
			"/containers/{name:.*}/changes": getContainersChanges,
			"/containers":                   getContainers,
			"/images/{name:.*}/json":        getImagesByName,
			"/containers/{name:.*}/json":    getContainersByName,
		},
		"POST": {
			"/auth": postAuth,
			"/containers/{name:.*}/kill":    postContainersKill,
			"/images/{name:.*}/tag":         postImagesTag,
			"/commit":                       postCommit,
			"/images":                       postImages,
			"/images/{name:*.}/insert":      postImagesInsert,
			"/images/{name:*.}/push":        postImagesPush,
			"/build":                        postBuild,
			"/containers":                   postContainers,
			"/containers/{name:.*}/restart": postContainersRestart,
			"/containers/{name:.*}/start":   postContainersStart,
			"/containers/{name:.*}/stop":    postContainersStop,
			"/containers/{name:.*}/wait":    postContainersWait,
			"/containers/{name:.*}/attach":  postContainersAttach,
		},
		"DELETE": {
			"/containers/{name:.*}": deleteContainers,
			"/images/{name:.*}":     deleteImages,
		},
	}

	for method, routes := range m {
		for route, fct := range routes {
			Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localMethod := method
			localFct := fct
			r.Path(localRoute).Methods(localMethod).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Debugf("Calling %s %s", localMethod, localRoute)
				log.Println(r.Method, r.RequestURI)

				if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
					userAgent := strings.Split(r.Header.Get("User-Agent"), "/")
					if len(userAgent) == 2 && userAgent[1] != VERSION {
						Debugf("Warning: client and server don't have the same version (client: %s, server: %s)", userAgent[1], VERSION)
					}
				}
				json, err := localFct(srv, w, r)
				if err != nil {
					httpError(w, err)
				}
				if json != nil {
					w.Header().Set("Content-Type", "application/json")
					w.Write(json)
				}
			})
		}
	}

	return http.ListenAndServe(addr, r)
}
