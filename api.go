package docker

import (
	"github.com/gorilla/mux"
	"net/http"
	_"encoding/json"
)


type RestEndpoint struct {
	*mux.Router
	runtime	*Runtime
}

func NewRestEndpoint(runtime *Runtime) *RestEndpoint {
	endpoint := &RestEndpoint{
		Router:	mux.NewRouter(),
		runtime: runtime,
	}
	endpoint.Path("/images").Methods("GET").HandlerFunc(endpoint.GetImages)
	endpoint.Path("/images").Methods("POST").HandlerFunc(endpoint.PostImages)
	endpoint.Path("/images/{id}").Methods("GET").HandlerFunc(endpoint.GetImage)
	endpoint.Path("/images/{id}").Methods("DELETE").HandlerFunc(endpoint.DeleteImage)
	endpoint.Path("/containers").Methods("GET").HandlerFunc(endpoint.GetContainers)
	endpoint.Path("/containers").Methods("POST").HandlerFunc(endpoint.PostContainers)
	endpoint.Path("/containers/{id}").Methods("GET").HandlerFunc(endpoint.GetContainer)
	endpoint.Path("/containers/{id}").Methods("DELETE").HandlerFunc(endpoint.DeleteContainer)
	return endpoint
}

func (ep *RestEndpoint) GetImages(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) PostImages(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) GetImage(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) DeleteImage(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) GetContainers(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) PostContainers(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) GetContainer(w http.ResponseWriter, r *http.Response) {

}

func (ep *RestEndpoint) DeleteContainer(w http.ResponseWriter, r *http.Response) {

}


