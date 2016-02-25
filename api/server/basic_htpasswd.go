// +build linux,daemon

package server

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
)

type htpasswd struct {
	Scheme  string
	Name    string
	realm   string
	command string
	file    string
}

func (s *htpasswd) GetChallenge(w http.ResponseWriter, r *http.Request) error {
	w.Header().Add("WWW-Authenticate", "Basic realm=\""+s.realm+"\"")
	return nil
}

func (s *htpasswd) CheckResponse(w http.ResponseWriter, r *http.Request) (User, error) {
	ah := r.Header["Authorization"]
	for _, h := range ah {
		fields := strings.SplitN(strings.Replace(h, "\t", " ", -1), " ", 2)
		if strings.ToLower(fields[0]) == "basic" {
			user, pass, ok := r.BasicAuth()
			if !ok {
				logrus.Errorf("error decoding Basic creds: \"%s\"", fields[1])
				return User{}, nil
			}
			cmd := exec.Command(s.command, "-v", "-i", s.file, user)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				logrus.Errorf("error preparing to run htpasswd: %v", err)
				return User{}, nil
			}
			err = cmd.Start()
			if err != nil {
				logrus.Errorf("error starting htpasswd: %v", err)
				return User{}, nil
			}
			io.WriteString(stdin, pass)
			stdin.Close()
			err = cmd.Wait()
			if err != nil {
				logrus.Errorf("error result from htpasswd: %v, failed Basic auth", err)
				return User{}, nil
			}
			return User{Name: user, Scheme: s.Scheme}, nil
		}
	}
	return User{}, nil
}

func createHtpasswd(options map[string]string) Authenticator {
	realm, ok := options["realm"]
	if !ok || realm == "" {
		realm = "localhost"
	}
	passwdfile, ok := options["htpasswd"]
	if !ok || passwdfile == "" {
		logrus.Debugf("htpasswd authentication option not set, not offering Basic auth via htpasswd file")
		return nil
	}
	cmd, err := exec.LookPath("htpasswd")
	if err != nil {
		logrus.Errorf("error locating htpasswd command: %v", err)
		return nil
	}
	return &htpasswd{Scheme: "Basic", Name: "htpasswd", realm: realm, command: cmd, file: passwdfile}
}

func validateHtpasswdOptions(option string) (string, error) {
	if strings.HasPrefix(option, "realm=") ||
		strings.HasPrefix(option, "htpasswd=") {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	RegisterAuthenticator(createHtpasswd)
	opts.RegisterAuthnOptionValidater(validateHtpasswdOptions)
}
