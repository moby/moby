package docker

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func ListenAndServe(addr string, rtime *Runtime) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	r.Path("/version").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		m := VersionOut{VERSION, GIT_COMMIT, NO_MEMORY_LIMIT}
		b, err := json.Marshal(m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/kill").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
                name := vars["name"]
                if container := rtime.Get(name); container != nil {
                        if err := container.Kill(); err != nil {
                                http.Error(w, "Error restarting container "+name+": "+err.Error(), http.StatusInternalServerError)
                                return
                        }
                } else {
                        http.Error(w, "No such container: "+name, http.StatusInternalServerError)
                        return
                }
                w.WriteHeader(200)
	})

	r.Path("/images").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		All := r.Form.Get("all")
		NameFilter :=  r.Form.Get("filter")
		Quiet :=  r.Form.Get("quiet")

		var allImages map[string]*Image
		var err error
		if All == "true" {
			allImages, err = rtime.graph.Map()
		} else {
			allImages, err = rtime.graph.Heads()
		}
		if err != nil {
			w.WriteHeader(500)
			return
		}
		var outs []ImagesOut
		for name, repository := range rtime.repositories.Repositories {
			if NameFilter != "" && name != NameFilter {
				continue
			}
			for tag, id := range repository {
				var out ImagesOut
				image, err := rtime.graph.Get(id)
				if err != nil {
					log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
					continue
				}
				delete(allImages, id)
				if Quiet != "true" {
					out.Repository = name
					out.Tag = tag
					out.Id = TruncateId(id)
					out.Created = HumanDuration(time.Now().Sub(image.Created)) + " ago"
				} else {
					out.Id = image.ShortId()
				}
				outs = append(outs, out)
			}
		}
		// Display images which aren't part of a
		if NameFilter == "" {
			for id, image := range allImages {
				var out ImagesOut
				if Quiet != "true" {
					out.Repository = "<none>"
					out.Tag = "<none>"
					out.Id = TruncateId(id)
					out.Created = HumanDuration(time.Now().Sub(image.Created)) + " ago"
				} else {
					out.Id = image.ShortId()
				}
				outs = append(outs, out)
			}
		}

		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/info").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		images, _ := rtime.graph.All()
		var imgcount int
		if images == nil {
			imgcount = 0
		} else {
			imgcount = len(images)
		}
		var out InfoOut
		out.Containers = len(rtime.List())
		out.Version = VERSION
		out.Images = imgcount
		if os.Getenv("DEBUG") == "1" {
			out.Debug = true
			out.NFd = getTotalUsedFds()
			out.NGoroutines = runtime.NumGoroutine()
		}
		b, err := json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/images/{name:.*}/history").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]

		image, err := rtime.repositories.LookupImage(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var outs []HistoryOut
		err = image.WalkHistory(func(img *Image) error {
			var out HistoryOut
			out.Id = rtime.repositories.ImageName(img.ShortId())
			out.Created = HumanDuration(time.Now().Sub(img.Created)) + " ago"
			out.CreatedBy = strings.Join(img.ContainerConfig.Cmd, " ")
			return nil
		})

		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/logs").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]

		if container := rtime.Get(name); container != nil {
			var out LogsOut

			logStdout, err := container.ReadLog("stdout")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			logStderr, err := container.ReadLog("stderr")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			stdout, errStdout := ioutil.ReadAll(logStdout)
			if errStdout != nil {
				http.Error(w, errStdout.Error(), http.StatusInternalServerError)
				return
			} else {
				out.Stdout = fmt.Sprintf("%s", stdout)
			}
			stderr, errStderr := ioutil.ReadAll(logStderr)
			if errStderr != nil {
				http.Error(w, errStderr.Error(), http.StatusInternalServerError)
				return
			} else {
				out.Stderr = fmt.Sprintf("%s", stderr)
			}

			b, err := json.Marshal(out)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				w.Write(b)
			}

		} else {
			http.Error(w, "No such container: "+name, http.StatusInternalServerError)
		}
	})

	r.Path("/containers").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		All := r.Form.Get("all")
		NoTrunc :=  r.Form.Get("notrunc")
		Quiet :=  r.Form.Get("quiet")
		Last := r.Form.Get("n")
		n, err := strconv.Atoi(Last)
		if err != nil {
			n = -1
		}
		var outs []PsOut

		for i, container := range rtime.List() {
			if !container.State.Running && All != "true" && n == -1 {
				continue
			}
			if i == n {
				break
			}
			var out PsOut
			out.Id = container.ShortId()
			if Quiet != "true" {
				command := fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
				if NoTrunc != "true" {
					command = Trunc(command, 20)
				}
				out.Image = rtime.repositories.ImageName(container.Image)
				out.Command = command
				out.Created = HumanDuration(time.Now().Sub(container.Created)) + " ago"
				out.Status = container.State.String()
			}
			outs = append(outs, out)
		}

		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/containers/{name:.*}/restart").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if container := rtime.Get(name); container != nil {
			if err := container.Restart(); err != nil {
				http.Error(w, "Error restarting container "+name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "No such container: "+name, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	})

	r.Path("/containers/{name:.*}").Methods("DELETE").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if container := rtime.Get(name); container != nil {
			if err := rtime.Destroy(container); err != nil {
				http.Error(w, "Error destroying container "+name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "No such container: "+name, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	})

	r.Path("/images/{name:.*}").Methods("DELETE").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		
		img, err := rtime.repositories.LookupImage(name)
		if err != nil {
			http.Error(w, "No such image: "+name, http.StatusInternalServerError)
			return
		} else {
			if err := rtime.graph.Delete(img.Id); err != nil {
				http.Error(w, "Error deleting image "+name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(200)
	})

	r.Path("/containers/{name:.*}/start").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if container := rtime.Get(name); container != nil {
			if err := container.Start(); err != nil {
				http.Error(w, "Error starting container "+name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "No such container: "+name, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	})

	r.Path("/containers/{name:.*}/stop").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		vars := mux.Vars(r)
		name := vars["name"]
		if container := rtime.Get(name); container != nil {
			if err := container.Stop(); err != nil {
				http.Error(w, "Error stopping container "+name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "No such container: "+name, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	})

	return http.ListenAndServe(addr, r)
}
