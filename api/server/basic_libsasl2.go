// +build linux,cgo,!static_build,daemon,libsasl2

package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
)

// #cgo LDFLAGS: -lsasl2
// #include <sys/types.h>
// #include <stdlib.h>
// #include <sasl/sasl.h>
//static char sasl_service[] = "docker";
//char *docker_sasl_service = sasl_service;
import "C"
import "unsafe"

type saslauthd struct {
	Scheme string
	Name   string
	realm  string
	crealm *C.char
}

func (s *saslauthd) GetChallenge(w http.ResponseWriter, r *http.Request) error {
	var conn *C.sasl_conn_t
	result := C.sasl_server_new(C.docker_sasl_service, nil, s.crealm, nil, nil, nil, 0, &conn)
	if result == C.SASL_OK {
		w.Header().Add("WWW-Authenticate", "Basic realm=\""+s.realm+"\"")
		C.sasl_dispose(&conn)
	} else {
		logrus.Errorf("error initializing libsasl (%d), not offering Basic auth", result)
	}
	return nil
}

func (s *saslauthd) CheckResponse(w http.ResponseWriter, r *http.Request) (User, error) {
	var conn *C.sasl_conn_t
	ah := r.Header["Authorization"]
	for _, h := range ah {
		fields := strings.SplitN(strings.Replace(h, "\t", " ", -1), " ", 2)
		if strings.ToLower(fields[0]) == "basic" {
			user, pass, ok := r.BasicAuth()
			if !ok {
				logrus.Errorf("error decoding Basic creds: \"%s\"", fields[1])
				return User{}, fmt.Errorf("error decoding Basic creds: \"%s\"", fields[1])
			}
			//logrus.Errorf("Basic: %s", fields[1])
			if C.sasl_server_new(C.docker_sasl_service, nil, s.crealm, nil, nil, nil, 0, &conn) == C.SASL_OK {
				defer C.sasl_dispose(&conn)
				cuser := C.CString(user)
				defer C.free(unsafe.Pointer(cuser))
				cpass := C.CString(pass)
				defer C.free(unsafe.Pointer(cpass))
				if C.sasl_checkpass(conn, cuser, C.uint(len(user)), cpass, C.uint(len(pass))) == C.SASL_OK {
					return User{Name: user, Scheme: s.Scheme}, nil
				}
				logrus.Errorf("failed Basic auth")
			}
			return User{}, nil
		}
	}
	return User{}, nil
}

func createLibsasl(options map[string]string) Authenticator {
	realm, ok := options["realm"]
	if !ok || realm == "" {
		realm = "localhost"
	}
	enabled, ok := options["libsasl2"]
	if !ok {
		logrus.Debugf("--authn-opt libsasl2=true option not specified, not checking Basic auth via libsasl2")
		return nil
	}
	if libsasl2, err := strconv.ParseBool(enabled); err != nil || !libsasl2 {
		logrus.Debugf("--authn-opt libsasl2 disabled, not checking Basic auth via libsasl2")
		return nil
	}
	result := C.sasl_server_init(nil, C.docker_sasl_service)
	if result != C.SASL_OK {
		logrus.Errorf("error %d initializing libsasl, not offering Basic auth via libsasl2", result)
		return nil
	}
	crealm := C.CString(realm)
	return &saslauthd{Scheme: "Basic", Name: "libsasl2", realm: realm, crealm: crealm}
}

func validateLibsasl2Options(option string) (string, error) {
	if strings.HasPrefix(option, "realm=") ||
		strings.HasPrefix(option, "libsasl2=") || option == "libsasl2" {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	RegisterAuthenticator(createLibsasl)
	opts.RegisterAuthnOptionValidater(validateLibsasl2Options)
}
