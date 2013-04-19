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
	"strings"
	"time"
)

func ListenAndServe(addr string, rtime *Runtime) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	r.Path("/version").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := VersionOut{VERSION, GIT_COMMIT, NO_MEMORY_LIMIT}
		b, err := json.Marshal(m)
		if err != nil {
			w.WriteHeader(500)
		} else {
			w.Write(b)
		}
	})

	r.Path("/kill").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			w.WriteHeader(500)
			return
		}

		var ret SimpleMessage
		for _, name := range ids {
			container := rtime.Get(name)
			if container == nil {
				ret.Message = "No such container: " + name + "\n"
				break
			}
			if err := container.Kill(); err != nil {
				ret.Message = ret.Message + "Error killing container " + name + ": " + err.Error() + "\n"
			}
		}
		if ret.Message == "" {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(500)
		}

		b, err := json.Marshal(ret)
		if err != nil {
			w.WriteHeader(500)
		} else {
			w.Write(b)
		}

	})

	r.Path("/images").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in ImagesIn
		json.NewDecoder(r.Body).Decode(&in)

		var allImages map[string]*Image
		var err error
		if in.All {
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
			if in.NameFilter != "" && name != in.NameFilter {
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
				if !in.Quiet {
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
		if in.NameFilter == "" {
			for id, image := range allImages {
				var out ImagesOut
				if !in.Quiet {
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

	r.Path("/info").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	r.Path("/history").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)

		var in HistoryIn
		json.NewDecoder(r.Body).Decode(&in)

		image, err := rtime.repositories.LookupImage(in.Name)
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

	r.Path("/logs").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var in LogsIn
		json.NewDecoder(r.Body).Decode(&in)

		if container := rtime.Get(in.Name); container != nil {
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
			http.Error(w, "No such container: "+in.Name, http.StatusInternalServerError)
		}
	})

	r.Path("/ps").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in PsIn
		json.NewDecoder(r.Body).Decode(&in)

		var outs []PsOut

		for i, container := range rtime.List() {
			if !container.State.Running && !in.All && in.Last == -1 {
				continue
			}
			if i == in.Last {
				break
			}
			var out PsOut
			out.Id = container.ShortId()
			if !in.Quiet {
				command := fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
				if !in.Full {
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

	r.Path("/restart").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ins, outs []string
		json.NewDecoder(r.Body).Decode(&ins)

		for _, name := range ins {
			if container := rtime.Get(name); container != nil {
				if err := container.Restart(); err != nil {
					http.Error(w, "Error restaring container "+name+": "+err.Error(), http.StatusInternalServerError)
					return
				}
				outs = append(outs, container.ShortId())
			} else {
				http.Error(w, "No such container: "+name, http.StatusInternalServerError)
				return
			}
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}
	})

	r.Path("/rm").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ins, outs []string
		json.NewDecoder(r.Body).Decode(&ins)

		for _, name := range ins {
			if container := rtime.Get(name); container != nil {
				if err := rtime.Destroy(container); err != nil {
					http.Error(w, "Error destroying container "+name+": "+err.Error(), http.StatusInternalServerError)
					return
				}
				outs = append(outs, container.ShortId())
			} else {
				http.Error(w, "No such container: "+name, http.StatusInternalServerError)
				return
			}
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}

	})

	r.Path("/rmi").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ins, outs []string
		json.NewDecoder(r.Body).Decode(&ins)

		for _, name := range ins {
			img, err := rtime.repositories.LookupImage(name)
			if err != nil {
				http.Error(w, "No such image: "+name, http.StatusInternalServerError)
				return
			} else {
				if err := rtime.graph.Delete(img.Id); err != nil {
					http.Error(w, "Error deleting image "+name+": "+err.Error(), http.StatusInternalServerError)
					return
				}
				outs = append(outs, img.ShortId())
			}
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}

	})

	r.Path("/run").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var config Config
		json.NewDecoder(r.Body).Decode(&config)

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		//TODO  config.Tty

		// Create new container                                         
		container, err := rtime.Create(&config)
		if err != nil {
			// If container not found, try to pull it                  
			if rtime.graph.IsNotExist(err) {
				bufrw.WriteString("Image " + config.Image + " not found, trying to pull it from registry.\r\n")
				bufrw.Flush()
				//TODO if err = srv.CmdPull(stdin, stdout, config.Image); err != nil {
				//return err
				//}
				if container, err = rtime.Create(&config); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		container = container
	})

	r.Path("/start").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ins, outs []string
		json.NewDecoder(r.Body).Decode(&ins)

		for _, name := range ins {
			if container := rtime.Get(name); container != nil {
				if err := container.Start(); err != nil {
					http.Error(w, "Error starting container "+name+": "+err.Error(), http.StatusInternalServerError)
					return
				}
				outs = append(outs, container.ShortId())
			} else {
				http.Error(w, "No such container: "+name, http.StatusInternalServerError)
				return
			}
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}

	})

	r.Path("/stop").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ins, outs []string
		json.NewDecoder(r.Body).Decode(&ins)

		for _, name := range ins {
			if container := rtime.Get(name); container != nil {
				if err := container.Stop(); err != nil {
					http.Error(w, "Error stopping container "+name+": "+err.Error(), http.StatusInternalServerError)
					return
				}
				outs = append(outs, container.ShortId())
			} else {
				http.Error(w, "No such container: "+name, http.StatusInternalServerError)
				return
			}
		}
		b, err := json.Marshal(outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(b)
		}

	})

	return http.ListenAndServe(addr, r)
}
