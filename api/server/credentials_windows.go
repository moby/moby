// +build windows

package server

import "net/http"

// Audit and system logging are unsupported in windows environments
func (s *Server) LogAction(w http.ResponseWriter, r *http.Request) error {
	return nil
}
