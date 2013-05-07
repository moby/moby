package docker

import (
	_ "bytes"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/gorilla/mux"
	"github.com/shin-/cookiejar"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func hijackServer(w http.ResponseWriter) (*os.File, net.Conn, error) {
	rwc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}

	file, err := rwc.(*net.TCPConn).File()
	if err != nil {
		return nil, rwc, err
	}

	// Flush the options to make sure the client sets the raw mode
	rwc.Write([]byte{})

	return file, rwc, nil
}

func httpError(w http.ResponseWriter, err error) {
	if strings.HasPrefix(err.Error(), "No such") {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ListenAndServe(addr string, srv *Server) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	r.Path("/auth").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		var out auth.AuthConfig
		out.Username = srv.runtime.authConfig.Username
		out.Email = srv.runtime.authConfig.Email
		b, err := json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/auth").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		var config auth.AuthConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if config.Username == srv.runtime.authConfig.Username {
			config.Password = srv.runtime.authConfig.Password
		}

		newAuthConfig := auth.NewAuthConfig(config.Username, config.Password, config.Email, srv.runtime.root)
		status, err := auth.Login(newAuthConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else {
			srv.runtime.graph.getHttpClient().Jar = cookiejar.NewCookieJar()
			srv.runtime.authConfig = newAuthConfig
		}
		if status != "" {
			b, err := json.Marshal(ApiAuth{status})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Write(b)
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/version").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		m := srv.DockerVersion()
		b, err := json.Marshal(m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/kill").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if err := srv.ContainerKill(name); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/containers/{name:.*}/export").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ContainerExport(name, file); err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
	})

	r.Path("/images").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		viz := r.Form.Get("viz")
		if viz == "1" {
			file, rwc, err := hijackServer(w)
			if file != nil {
				defer file.Close()
			}
			if rwc != nil {
				defer rwc.Close()
			}
			if err != nil {
				httpError(w, err)
				return
			}
			fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
			if err := srv.ImagesViz(file); err != nil {
				fmt.Fprintln(file, "Error: "+err.Error())
			}
			return
		}

		all := r.Form.Get("all")
		filter := r.Form.Get("filter")
		quiet := r.Form.Get("quiet")

		outs, err := srv.Images(all, filter, quiet)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/info").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		out := srv.DockerInfo()
		b, err := json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/images/{name:.*}/history").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		outs, err := srv.ImageHistory(name)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/changes").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		changesStr, err := srv.ContainerChanges(name)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(changesStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/port").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vars := mux.Vars(r)
		name := vars["name"]
		out, err := srv.ContainerPort(name, r.Form.Get("port"))
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(ApiPort{out})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}

	})

	r.Path("/containers").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		all := r.Form.Get("all")
		notrunc := r.Form.Get("notrunc")
		quiet := r.Form.Get("quiet")
		n, err := strconv.Atoi(r.Form.Get("n"))
		if err != nil {
			n = -1
		}

		outs := srv.Containers(all, notrunc, quiet, n)
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/images/{name:.*}/tag").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		repo := r.Form.Get("repo")
		tag := r.Form.Get("tag")
		vars := mux.Vars(r)
		name := vars["name"]
		var force bool
		if r.Form.Get("force") == "1" {
			force = true
		}

		if err := srv.ContainerTag(name, repo, tag, force); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	r.Path("/commit").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		repo := r.Form.Get("repo")
		tag := r.Form.Get("tag")
		container := r.Form.Get("container")
		author := r.Form.Get("author")
		comment := r.Form.Get("comment")
		id, err := srv.ContainerCommit(container, repo, tag, author, comment, &config)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(ApiId{id})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/images").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		src := r.Form.Get("fromSrc")
		image := r.Form.Get("fromImage")
		repo := r.Form.Get("repo")
		tag := r.Form.Get("tag")

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if image != "" { //pull
			registry := r.Form.Get("registry")
			if err := srv.ImagePull(image, tag, registry, file); err != nil {
				fmt.Fprintln(file, "Error: "+err.Error())
			}
		} else { //import
			if err := srv.ImageImport(src, repo, tag, file); err != nil {
				fmt.Fprintln(file, "Error: "+err.Error())
			}
		}
	})

	r.Path("/images/{name:*.}/insert").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		url := r.Form.Get("url")
		path := r.Form.Get("path")
		vars := mux.Vars(r)
		name := vars["name"]

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ImageInsert(name, url, path, file); err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
	})
	r.Path("/images/{name:*.}/push").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		registry := r.Form.Get("registry")

		vars := mux.Vars(r)
		name := vars["name"]

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ImagePush(name, registry, file); err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
	})

	r.Path("/build").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ImageCreateFormFile(file); err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
	})

	r.Path("/containers").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id, memoryW, swapW, err := srv.ContainerCreate(config)
		if err != nil {
			httpError(w, err)
			return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/restart").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil || t < 0 {
			t = 10
		}
		vars := mux.Vars(r)
		name := vars["name"]
		if err := srv.ContainerRestart(name, t); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/containers/{name:.*}").Methods("DELETE").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vars := mux.Vars(r)
		name := vars["name"]
		var v bool
		if r.Form.Get("v") == "1" {
			v = true
		}
		if err := srv.ContainerDestroy(name, v); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/images/{name:.*}").Methods("DELETE").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if err := srv.ImageDelete(name); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/containers/{name:.*}/start").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if err := srv.ContainerStart(name); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/containers/{name:.*}/stop").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil || t < 0 {
			t = 10
		}
		vars := mux.Vars(r)
		name := vars["name"]

		if err := srv.ContainerStop(name, t); err != nil {
			httpError(w, err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Path("/containers/{name:.*}/wait").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		status, err := srv.ContainerWait(name)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(ApiWait{status})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/attach").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logs := r.Form.Get("logs")
		stream := r.Form.Get("stream")
		stdin := r.Form.Get("stdin")
		stdout := r.Form.Get("stdout")
		stderr := r.Form.Get("stderr")
		vars := mux.Vars(r)
		name := vars["name"]

		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return
		}

		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, file); err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
	})

	r.Path("/containers/{name:.*}").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]

		container, err := srv.ContainerInspect(name)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(container)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	r.Path("/images/{name:.*}").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]

		image, err := srv.ImageInspect(name)
		if err != nil {
			httpError(w, err)
			return
		}
		b, err := json.Marshal(image)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})

	return http.ListenAndServe(addr, r)
}
