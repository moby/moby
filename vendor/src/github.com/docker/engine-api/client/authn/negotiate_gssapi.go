// +build linux,cgo,!static_build,gssapi

package authn

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unsafe"
)

// #cgo pkg-config: krb5-gssapi
// #include <sys/types.h>
// #include <stdlib.h>
// #include <gssapi/gssapi.h>
// gss_OID_desc SpnegoOid = {
//     .length = 6,
//     .elements = "\053\006\001\005\005\002"
// };
import "C"

type negotiate struct {
	logger   Logger
	major    C.OM_uint32
	ctx      C.gss_ctx_id_t
	hostname string
}

func getErrorDesc(major, minor C.OM_uint32, mech C.gss_OID) string {
	var mj, mn C.gss_buffer_desc
	var str string
	var cm, mc C.OM_uint32

	major = C.gss_display_status(&cm, major, C.GSS_C_GSS_CODE, nil, &mc, &mj)
	if major == 0 && mj.value != nil && mj.length > 0 {
		str = string(C.GoBytes(mj.value, C.int(mj.length)))
		C.gss_release_buffer(nil, &mj)
	}
	if minor != 0 {
		major = C.gss_display_status(&cm, minor, C.GSS_C_MECH_CODE, mech, &mc, &mn)
		if major == 0 && mn.value != nil && mn.length > 0 {
			if str != "" {
				str = str + ": " + string(C.GoBytes(mn.value, C.int(mn.length)))
			} else {
				str = "?: " + string(C.GoBytes(mn.value, C.int(mn.length)))
			}
			C.gss_release_buffer(nil, &mn)
		}
	}
	return str
}

func (n *negotiate) scheme() string {
	return "Negotiate"
}

func (n *negotiate) authStep(challenge string, req *http.Request) (result bool, err error) {
	var itoken, otoken, namebuf C.gss_buffer_desc
	var itokenptr *C.gss_buffer_desc
	var minor, lifetime, flags C.OM_uint32
	var name C.gss_name_t
	var mech C.gss_OID

	fields := strings.SplitN(strings.Replace(challenge, "\t", " ", -1), " ", 2)
	if strings.ToLower(fields[0]) == "negotiate" {
		// Decode any incoming token, and set a pointer to it, if there is one.
		if len(fields) > 1 {
			token, err := base64.StdEncoding.DecodeString(fields[1])
			if err != nil {
				n.logger.Error(fmt.Sprintf("error decoding Negotiate token from server: \"%s\"", fields[1]))
				return false, err
			}
			n.logger.Debug(fmt.Sprintf("gssapi: received token \"%s\"", fields[1]))
			itoken.value = unsafe.Pointer(&token[0])
			itoken.length = C.size_t(len(token))
			itokenptr = &itoken
		}
		// Work out the server's GSSAPI name.
		service := "HTTP@" + n.hostname
		n.logger.Debug(fmt.Sprintf("gssapi: using service name \"%s\"", service))
		// Import the name.
		namebuf.value = unsafe.Pointer(C.CString(service))
		defer C.free(namebuf.value)
		namebuf.length = C.size_t(len(service))
		mech = &C.SpnegoOid
		n.major = C.gss_import_name(&minor, &namebuf, C.GSS_C_NT_HOSTBASED_SERVICE, &name)
		if name != nil {
			defer C.gss_release_name(&minor, &name)
		}
		if n.major != C.GSS_S_COMPLETE {
			n.logger.Info(fmt.Sprintf("error importing server name (%s), not attempting Negotiate auth", getErrorDesc(n.major, minor, nil)))
			return false, nil
		}
		lifetime = C.GSS_C_INDEFINITE
		// Call the initiation function, maybe with data we got from the server.
		if itokenptr != nil {
			n.logger.Debug(fmt.Sprintf("gssapi: reading token (%d bytes)", itokenptr.length))
		}
		n.major = C.gss_init_sec_context(&minor, nil, &n.ctx, name, mech, flags, lifetime, nil, itokenptr, nil, &otoken, nil, nil)
		if otoken.length > 0 {
			defer C.gss_release_buffer(&minor, &otoken)
		}
		// Check for complete or partial success.
		if n.major != C.GSS_S_COMPLETE && n.major != C.GSS_S_CONTINUE_NEEDED {
			if itokenptr != nil {
				n.logger.Info(fmt.Sprintf("error generating GSSAPI session initiation token (%s), not attempting Negotiate auth", getErrorDesc(n.major, minor, nil)))
			} else {
				n.logger.Debug(fmt.Sprintf("error generating GSSAPI session initiation token (%s), not attempting Negotiate auth", getErrorDesc(n.major, minor, nil)))
			}
			return false, nil
		}
		// Format data for the server.
		if otoken.length > 0 {
			if req == nil {
				n.logger.Error("synchronization error: got unexpected Negotiate token to send to server")
				return false, nil
			}
			response := C.GoBytes(otoken.value, C.int(otoken.length))
			token := base64.StdEncoding.EncodeToString(response)
			req.Header.Add("Authorization", "Negotiate "+token)
			n.logger.Debug(fmt.Sprintf("gssapi: generated token \"%s\"", token))
		}
		if n.major == C.GSS_S_CONTINUE_NEEDED {
			n.logger.Debug("gssapi: continue needed")
		} else {
			n.logger.Debug("gssapi: complete")
		}
		return true, nil
	}
	return false, nil
}

func (n *negotiate) authRespond(challenge string, req *http.Request) (result bool, err error) {
	if n.hostname == "" {
		serverhost := req.Host
		if serverhost == "" {
			serverhost = req.URL.Host
		}
		if len(serverhost) == 0 || serverhost[0] == os.PathSeparator {
			serverhost, _ = os.Hostname()
		}
		sep := strings.LastIndex(serverhost, ":")
		if sep > -1 && sep > strings.LastIndex(serverhost, "]") {
			serverhost = serverhost[:sep]
		}
		n.hostname = serverhost
	}
	return n.authStep(challenge, req)
}

func (n *negotiate) authCompleted(challenge string, resp *http.Response) (completed bool, err error) {
	var minor C.OM_uint32
	completed, err = n.authStep(challenge, nil)
	// Reset state, in case we need to do all of this again.
	if n.ctx != nil {
		C.gss_delete_sec_context(&minor, &n.ctx, nil)
		n.ctx = nil
	}
	return
}

func createNegotiate(logger Logger, authers []interface{}) authResponder {
	for _, auther := range authers {
		if b, ok := auther.(NegotiateAuther); ok {
			if b != nil && b.GetNegotiateAuth() {
				return &negotiate{logger: logger}
			}
		}
	}
	return nil
}

func init() {
	authResponderCreators = append(authResponderCreators, createNegotiate)
}
