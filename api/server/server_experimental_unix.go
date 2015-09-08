// +build experimental,!windows

package server

func (s *Server) registerNetworkRouter() {
	ws := s.newWebService()

	handler := restfulHTTPRoute(s.daemon.NetworkAPIRouter())

	networkRouterMethods := []string{"GET", "POST", "PUT", "DELETE"}
	networkRouterPaths := []string{"/networks", "/services"}

	for _, method := range networkRouterMethods {
		for _, route := range networkRouterPaths {
			ws.Route(ws.Method(method).Path(versionPath + route).To(handler))
			ws.Route(ws.Method(method).Path(route).To(handler))
		}
	}

	s.router.Add(ws)
}
