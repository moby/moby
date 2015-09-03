package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func boolValue(r formParser, k string) bool {
	s := strings.ToLower(strings.TrimSpace(r.FormValue(k)))
	return !(s == "" || s == "0" || s == "no" || s == "false" || s == "none")
}

// boolValueOrDefault returns the default bool passed if the query param is
// missing, otherwise it's just a proxy to boolValue above
func boolValueOrDefault(r *http.Request, k string, d bool) bool {
	if _, ok := r.Form[k]; !ok {
		return d
	}
	return boolValue(r, k)
}

func int64ValueOrZero(r formParser, k string) int64 {
	val, err := strconv.ParseInt(r.FormValue(k), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func parsePathParameter(r formParser) (string, error) {
	if err := parseForm(r); err != nil {
		return "", err
	}
	path := filepath.FromSlash(r.FormValue("path"))
	if path == "" {
		return "", fmt.Errorf("bad parameter: 'path' cannot be empty")
	}
	return path, nil
}
