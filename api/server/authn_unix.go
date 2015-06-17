// +build linux,cgo,!gccgo

package server

import (
	"fmt"
	"net/http"
	"os/user"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
)

func getUserFromHTTPResponseWriter(w http.ResponseWriter, options map[string]string) User {
	// First, check that we're even supposed to be looking up anything.
	local, ok := options["local-auth"]
	if !ok {
		return User{}
	}
	if use, err := strconv.ParseBool(local); err != nil || !use {
		return User{}
	}
	wi := reflect.ValueOf(w)
	// Dereference the http.ResponseWriter interface to look at the struct...
	hr := wi.Elem()
	switch hr.Kind() {
	case reflect.Struct:
	default:
		logrus.Warn("ResponseWriter is not a struct")
		return User{}
	}
	// which is an http.response that contains a field named "conn"...
	c := hr.FieldByName("conn")
	if !c.IsValid() {
		logrus.Warn("ResponseWriter has no conn field")
		return User{}
	}
	// ... which is an http.conn, which is an interface ...
	hc := c.Elem()
	switch hc.Kind() {
	case reflect.Struct:
	default:
		logrus.Warn("conn is not an interface to a struct: " + c.Elem().Kind().String())
		return User{}
	}
	// ... and which has an element named "rwc" ...
	rwc := hc.FieldByName("rwc")
	if !rwc.IsValid() {
		logrus.Warn("conn has no rwc field")
		return User{}
	}
	// ... which is a pointer to a net.Conn, which is an interface ...
	nc := rwc.Elem()
	// ... to a net.UnixConn structure ...
	nuc := nc.Elem()
	switch nuc.Kind() {
	case reflect.Struct:
	default:
		logrus.Warn("rwc is not an interface to a struct: " + rwc.Elem().Kind().String())
		return User{}
	}
	// ... which contains a net.conn named "fd" ...
	fd := nuc.FieldByName("fd")
	if !fd.IsValid() {
		logrus.Warn("rwc has no fd field")
		return User{}
	}
	// ... which is a pointer to a net.netFD structure ...
	nfd := fd.Elem()
	switch nfd.Kind() {
	case reflect.Struct:
	default:
		logrus.Warn("fd is not a struct")
		return User{}
	}
	// ... which contains an integer named sysfd.
	sysfd := nfd.FieldByName("sysfd")
	if !sysfd.IsValid() {
		logrus.Warn("fd has no sysfd field")
		return User{}
	}
	// read the address of the local end of the socket
	sa, err := syscall.Getsockname(int(sysfd.Int()))
	if err != nil {
		logrus.Warn("error reading server socket address")
		return User{}
	}
	// and only try to read the user if it's a Unix socket
	if _, isUnix := sa.(*syscall.SockaddrUnix); !isUnix {
		logrus.Warn("error reading server socket address")
		return User{}
	}
	uc, err := syscall.GetsockoptUcred(int(sysfd.Int()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil || uc == nil {
		logrus.Warnf("%v: error reading client identity from kernel", err)
		return User{}
	}
	uidstr := fmt.Sprintf("%d", uc.Uid)
	pwd, err := user.LookupId(uidstr)
	if err != nil || pwd == nil {
		logrus.Warnf("unable to look up UID %s: %v", uidstr, err)
		return User{HaveUID: true, UID: uc.Uid}
	}
	logrus.Debugf("read UID %s (%s) from kernel", uidstr, pwd.Username)
	return User{Name: pwd.Username, HaveUID: true, UID: uc.Uid, Scheme: "External"}
}

func validateLocalAuthOption(option string) (string, error) {
	if strings.HasPrefix(option, "local-auth=") || option == "local-auth" {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	opts.RegisterAuthnOptionValidater(validateLocalAuthOption)
}
