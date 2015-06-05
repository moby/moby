package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/types"
	"github.com/gorilla/mux"
)

var (
	successResponse  = responseStatus{Status: "Success", StatusCode: http.StatusOK}
	createdResponse  = responseStatus{Status: "Created", StatusCode: http.StatusCreated}
	mismatchResponse = responseStatus{Status: "Body/URI parameter mismatch", StatusCode: http.StatusBadRequest}
	badQueryresponse = responseStatus{Status: "Unsupported query", StatusCode: http.StatusBadRequest}
)

const (
	// Resource name regex
	regex = "[a-zA-Z_0-9-]+"
	// Router URL variable definition
	nwName = "{" + urlNwName + ":" + regex + "}"
	nwID   = "{" + urlNwID + ":" + regex + "}"
	nwPID  = "{" + urlNwPID + ":" + regex + "}"
	epName = "{" + urlEpName + ":" + regex + "}"
	epID   = "{" + urlEpID + ":" + regex + "}"
	epPID  = "{" + urlEpPID + ":" + regex + "}"
	cnID   = "{" + urlCnID + ":" + regex + "}"

	// Internal URL variable name, they can be anything
	urlNwName = "network-name"
	urlNwID   = "network-id"
	urlNwPID  = "network-partial-id"
	urlEpName = "endpoint-name"
	urlEpID   = "endpoint-id"
	urlEpPID  = "endpoint-partial-id"
	urlCnID   = "container-id"
)

// NewHTTPHandler creates and initialize the HTTP handler to serve the requests for libnetwork
func NewHTTPHandler(c libnetwork.NetworkController) func(w http.ResponseWriter, req *http.Request) {
	h := &httpHandler{c: c}
	h.initRouter()
	return h.handleRequest
}

type responseStatus struct {
	Status     string
	StatusCode int
}

func (r *responseStatus) isOK() bool {
	return r.StatusCode == http.StatusOK || r.StatusCode == http.StatusCreated
}

type processor func(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus)

type httpHandler struct {
	c libnetwork.NetworkController
	r *mux.Router
}

func (h *httpHandler) handleRequest(w http.ResponseWriter, req *http.Request) {
	// Make sure the service is there
	if h.c == nil {
		http.Error(w, "NetworkController is not available", http.StatusServiceUnavailable)
		return
	}

	// Get handler from router and execute it
	h.r.ServeHTTP(w, req)
}

func (h *httpHandler) initRouter() {
	m := map[string][]struct {
		url string
		qrs []string
		fct processor
	}{
		"GET": {
			// Order matters
			{"/networks", []string{"name", nwName}, procGetNetworks},
			{"/networks", []string{"partial-id", nwPID}, procGetNetworks},
			{"/networks", nil, procGetNetworks},
			{"/networks/" + nwID, nil, procGetNetwork},
			{"/networks/" + nwID + "/endpoints", []string{"name", epName}, procGetEndpoints},
			{"/networks/" + nwID + "/endpoints", []string{"partial-id", epPID}, procGetEndpoints},
			{"/networks/" + nwID + "/endpoints", nil, procGetEndpoints},
			{"/networks/" + nwID + "/endpoints/" + epID, nil, procGetEndpoint},
		},
		"POST": {
			{"/networks", nil, procCreateNetwork},
			{"/networks/" + nwID + "/endpoints", nil, procCreateEndpoint},
			{"/networks/" + nwID + "/endpoints/" + epID + "/containers", nil, procJoinEndpoint},
		},
		"DELETE": {
			{"/networks/" + nwID, nil, procDeleteNetwork},
			{"/networks/" + nwID + "/endpoints/" + epID, nil, procDeleteEndpoint},
			{"/networks/id/" + nwID + "/endpoints/" + epID + "/containers/" + cnID, nil, procLeaveEndpoint},
		},
	}

	h.r = mux.NewRouter()
	for method, routes := range m {
		for _, route := range routes {
			r := h.r.Path("/{.*}" + route.url).Methods(method).HandlerFunc(makeHandler(h.c, route.fct))
			if route.qrs != nil {
				r.Queries(route.qrs...)
			}
		}
	}
}

func makeHandler(ctrl libnetwork.NetworkController, fct processor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var (
			body []byte
			err  error
		)
		if req.Body != nil {
			body, err = ioutil.ReadAll(req.Body)
			if err != nil {
				http.Error(w, "Invalid body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		res, rsp := fct(ctrl, mux.Vars(req), body)
		if !rsp.isOK() {
			http.Error(w, rsp.Status, rsp.StatusCode)
			return
		}
		if res != nil {
			writeJSON(w, rsp.StatusCode, res)
		}
	}
}

/*****************
 Resource Builders
******************/

func buildNetworkResource(nw libnetwork.Network) *networkResource {
	r := &networkResource{}
	if nw != nil {
		r.Name = nw.Name()
		r.ID = nw.ID()
		r.Type = nw.Type()
		epl := nw.Endpoints()
		r.Endpoints = make([]*endpointResource, 0, len(epl))
		for _, e := range epl {
			epr := buildEndpointResource(e)
			r.Endpoints = append(r.Endpoints, epr)
		}
	}
	return r
}

func buildEndpointResource(ep libnetwork.Endpoint) *endpointResource {
	r := &endpointResource{}
	if ep != nil {
		r.Name = ep.Name()
		r.ID = ep.ID()
		r.Network = ep.Network()
	}
	return r
}

/**************
 Options Parser
***************/

func (ej *endpointJoin) parseOptions() []libnetwork.EndpointOption {
	var setFctList []libnetwork.EndpointOption
	if ej.HostName != "" {
		setFctList = append(setFctList, libnetwork.JoinOptionHostname(ej.HostName))
	}
	if ej.DomainName != "" {
		setFctList = append(setFctList, libnetwork.JoinOptionDomainname(ej.DomainName))
	}
	if ej.HostsPath != "" {
		setFctList = append(setFctList, libnetwork.JoinOptionHostsPath(ej.HostsPath))
	}
	if ej.ResolvConfPath != "" {
		setFctList = append(setFctList, libnetwork.JoinOptionResolvConfPath(ej.ResolvConfPath))
	}
	if ej.UseDefaultSandbox {
		setFctList = append(setFctList, libnetwork.JoinOptionUseDefaultSandbox())
	}
	if ej.DNS != nil {
		for _, d := range ej.DNS {
			setFctList = append(setFctList, libnetwork.JoinOptionDNS(d))
		}
	}
	if ej.ExtraHosts != nil {
		for _, e := range ej.ExtraHosts {
			setFctList = append(setFctList, libnetwork.JoinOptionExtraHost(e.Name, e.Address))
		}
	}
	if ej.ParentUpdates != nil {
		for _, p := range ej.ParentUpdates {
			setFctList = append(setFctList, libnetwork.JoinOptionParentUpdate(p.EndpointID, p.Name, p.Address))
		}
	}
	return setFctList
}

/******************
 Process functions
*******************/

/***************************
 NetworkController interface
****************************/
func procCreateNetwork(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	var create networkCreate

	err := json.Unmarshal(body, &create)
	if err != nil {
		return "", &responseStatus{Status: "Invalid body: " + err.Error(), StatusCode: http.StatusBadRequest}
	}

	nw, err := c.NewNetwork(create.NetworkType, create.Name, nil)
	if err != nil {
		return "", convertNetworkError(err)
	}

	return nw.ID(), &createdResponse
}

func procGetNetwork(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	t, by := detectNetworkTarget(vars)
	nw, errRsp := findNetwork(c, t, by)
	if !errRsp.isOK() {
		return nil, errRsp
	}
	return buildNetworkResource(nw), &successResponse
}

func procGetNetworks(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	var list []*networkResource

	// Look for query filters and validate
	name, queryByName := vars[urlNwName]
	shortID, queryByPid := vars[urlNwPID]
	if queryByName && queryByPid {
		return nil, &badQueryresponse
	}

	if queryByName {
		if nw, errRsp := findNetwork(c, name, byName); errRsp.isOK() {
			list = append(list, buildNetworkResource(nw))
		}
	} else if queryByPid {
		// Return all the prefix-matching networks
		l := func(nw libnetwork.Network) bool {
			if strings.HasPrefix(nw.ID(), shortID) {
				list = append(list, buildNetworkResource(nw))
			}
			return false
		}
		c.WalkNetworks(l)
	} else {
		for _, nw := range c.Networks() {
			list = append(list, buildNetworkResource(nw))
		}
	}

	return list, &successResponse
}

/******************
 Network interface
*******************/
func procCreateEndpoint(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	var ec endpointCreate

	err := json.Unmarshal(body, &ec)
	if err != nil {
		return "", &responseStatus{Status: "Invalid body: " + err.Error(), StatusCode: http.StatusBadRequest}
	}

	nwT, nwBy := detectNetworkTarget(vars)
	n, errRsp := findNetwork(c, nwT, nwBy)
	if !errRsp.isOK() {
		return "", errRsp
	}

	var setFctList []libnetwork.EndpointOption
	if ec.ExposedPorts != nil {
		setFctList = append(setFctList, libnetwork.CreateOptionExposedPorts(ec.ExposedPorts))
	}
	if ec.PortMapping != nil {
		setFctList = append(setFctList, libnetwork.CreateOptionPortMapping(ec.PortMapping))
	}

	ep, err := n.CreateEndpoint(ec.Name, setFctList...)
	if err != nil {
		return "", convertNetworkError(err)
	}

	return ep.ID(), &createdResponse
}

func procGetEndpoint(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	nwT, nwBy := detectNetworkTarget(vars)
	epT, epBy := detectEndpointTarget(vars)

	ep, errRsp := findEndpoint(c, nwT, epT, nwBy, epBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	return buildEndpointResource(ep), &successResponse
}

func procGetEndpoints(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	// Look for query filters and validate
	name, queryByName := vars[urlEpName]
	shortID, queryByPid := vars[urlEpPID]
	if queryByName && queryByPid {
		return nil, &badQueryresponse
	}

	nwT, nwBy := detectNetworkTarget(vars)
	nw, errRsp := findNetwork(c, nwT, nwBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	var list []*endpointResource

	// If query parameter is specified, return a filtered collection
	if queryByName {
		if ep, errRsp := findEndpoint(c, nwT, name, nwBy, byName); errRsp.isOK() {
			list = append(list, buildEndpointResource(ep))
		}
	} else if queryByPid {
		// Return all the prefix-matching networks
		l := func(ep libnetwork.Endpoint) bool {
			if strings.HasPrefix(ep.ID(), shortID) {
				list = append(list, buildEndpointResource(ep))
			}
			return false
		}
		nw.WalkEndpoints(l)
	} else {
		for _, ep := range nw.Endpoints() {
			epr := buildEndpointResource(ep)
			list = append(list, epr)
		}
	}

	return list, &successResponse
}

func procDeleteNetwork(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	target, by := detectNetworkTarget(vars)

	nw, errRsp := findNetwork(c, target, by)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	err := nw.Delete()
	if err != nil {
		return nil, convertNetworkError(err)
	}

	return nil, &successResponse
}

/******************
 Endpoint interface
*******************/
func procJoinEndpoint(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	var ej endpointJoin
	err := json.Unmarshal(body, &ej)
	if err != nil {
		return nil, &responseStatus{Status: "Invalid body: " + err.Error(), StatusCode: http.StatusBadRequest}
	}

	nwT, nwBy := detectNetworkTarget(vars)
	epT, epBy := detectEndpointTarget(vars)

	ep, errRsp := findEndpoint(c, nwT, epT, nwBy, epBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	cd, err := ep.Join(ej.ContainerID, ej.parseOptions()...)
	if err != nil {
		return nil, convertNetworkError(err)
	}
	return cd, &successResponse
}

func procLeaveEndpoint(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	nwT, nwBy := detectNetworkTarget(vars)
	epT, epBy := detectEndpointTarget(vars)

	ep, errRsp := findEndpoint(c, nwT, epT, nwBy, epBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	err := ep.Leave(vars[urlCnID])
	if err != nil {
		return nil, convertNetworkError(err)
	}

	return nil, &successResponse
}

func procDeleteEndpoint(c libnetwork.NetworkController, vars map[string]string, body []byte) (interface{}, *responseStatus) {
	nwT, nwBy := detectNetworkTarget(vars)
	epT, epBy := detectEndpointTarget(vars)

	ep, errRsp := findEndpoint(c, nwT, epT, nwBy, epBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}

	err := ep.Delete()
	if err != nil {
		return nil, convertNetworkError(err)
	}

	return nil, &successResponse
}

/***********
  Utilities
************/
const (
	byID = iota
	byName
)

func detectNetworkTarget(vars map[string]string) (string, int) {
	if target, ok := vars[urlNwName]; ok {
		return target, byName
	}
	if target, ok := vars[urlNwID]; ok {
		return target, byID
	}
	// vars are populated from the URL, following cannot happen
	panic("Missing URL variable parameter for network")
}

func detectEndpointTarget(vars map[string]string) (string, int) {
	if target, ok := vars[urlEpName]; ok {
		return target, byName
	}
	if target, ok := vars[urlEpID]; ok {
		return target, byID
	}
	// vars are populated from the URL, following cannot happen
	panic("Missing URL variable parameter for endpoint")
}

func findNetwork(c libnetwork.NetworkController, s string, by int) (libnetwork.Network, *responseStatus) {
	var (
		nw  libnetwork.Network
		err error
	)
	switch by {
	case byID:
		nw, err = c.NetworkByID(s)
	case byName:
		nw, err = c.NetworkByName(s)
	default:
		panic(fmt.Sprintf("unexpected selector for network search: %d", by))
	}
	if err != nil {
		if _, ok := err.(libnetwork.ErrNoSuchNetwork); ok {
			return nil, &responseStatus{Status: "Resource not found: Network", StatusCode: http.StatusNotFound}
		}
		return nil, &responseStatus{Status: err.Error(), StatusCode: http.StatusBadRequest}
	}
	return nw, &successResponse
}

func findEndpoint(c libnetwork.NetworkController, ns, es string, nwBy, epBy int) (libnetwork.Endpoint, *responseStatus) {
	nw, errRsp := findNetwork(c, ns, nwBy)
	if !errRsp.isOK() {
		return nil, errRsp
	}
	var (
		err error
		ep  libnetwork.Endpoint
	)
	switch epBy {
	case byID:
		ep, err = nw.EndpointByID(es)
	case byName:
		ep, err = nw.EndpointByName(es)
	default:
		panic(fmt.Sprintf("unexpected selector for endpoint search: %d", epBy))
	}
	if err != nil {
		if _, ok := err.(libnetwork.ErrNoSuchEndpoint); ok {
			return nil, &responseStatus{Status: "Resource not found: Endpoint", StatusCode: http.StatusNotFound}
		}
		return nil, &responseStatus{Status: err.Error(), StatusCode: http.StatusBadRequest}
	}
	return ep, &successResponse
}

func convertNetworkError(err error) *responseStatus {
	var code int
	switch err.(type) {
	case types.BadRequestError:
		code = http.StatusBadRequest
	case types.ForbiddenError:
		code = http.StatusForbidden
	case types.NotFoundError:
		code = http.StatusNotFound
	case types.TimeoutError:
		code = http.StatusRequestTimeout
	case types.NotImplementedError:
		code = http.StatusNotImplemented
	case types.NoServiceError:
		code = http.StatusServiceUnavailable
	case types.InternalError:
		code = http.StatusInternalServerError
	default:
		code = http.StatusInternalServerError
	}
	return &responseStatus{Status: err.Error(), StatusCode: code}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}
