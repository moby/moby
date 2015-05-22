package server

import (
	"net/http"
	"strconv"
	"strings"
)

func boolValue(r *http.Request, k string) bool {
	s := strings.ToLower(strings.TrimSpace(r.FormValue(k)))
	return !(s == "" || s == "0" || s == "no" || s == "false" || s == "none")
}

func int64ValueOrZero(r *http.Request, k string) int64 {
	val, err := strconv.ParseInt(r.FormValue(k), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func prefixedValuesToMap(r *http.Request, prefix string) map[string]string {
	mapOfPrefix := make(map[string]string)
	query := r.URL.Query()
	for key := range query {
		if strings.HasPrefix(key, prefix) {
			label := key[len(prefix):]
			mapOfPrefix[label] = query.Get(key)
		}
	}
	return mapOfPrefix
}
