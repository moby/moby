// +build linux,cgo,!static_build,daemon,gssapi

package server

import (
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"unsafe"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
	"github.com/gorilla/sessions"
)

// #cgo pkg-config: krb5-gssapi
// #include <sys/types.h>
// #include <stdlib.h>
// #include <gssapi/gssapi.h>
// #include <gssapi/gssapi_krb5.h>
// static gss_OID_desc Krb5Oid = {
//     .length = 9,
//     .elements = "\052\206\110\206\367\022\001\002\002"
// };
// static gss_OID_desc Krb5OldOid = {
//     .length = 5,
//     .elements = "\053\005\001\005\002"
// };
// static gss_OID_desc Krb5WrongOid = {
//     .length = 9,
//     .elements = "\052\206\110\202\367\022\001\002\002"
// };
// static gss_OID_desc AllKrb5Oids[] = {
//    {
//     .length = 9,
//     .elements = "\052\206\110\206\367\022\001\002\002"
//    },
//    {
//     .length = 5,
//     .elements = "\053\005\001\005\002"
//    },
//    {
//     .length = 9,
//     .elements = "\052\206\110\202\367\022\001\002\002"
//    }
// };
// gss_OID_set_desc ServerSpnegoOidSet = {
//    .count = 3,
//    .elements = &AllKrb5Oids[0]
// };
import "C"

type negotiator struct {
	Scheme string
	Name   string
}

func getErrorDesc(major, minor C.OM_uint32, mech C.gss_OID) string {
	var mj, mn C.gss_buffer_desc
	var str string
	var cm, mc C.OM_uint32

	major = C.gss_display_status(&cm, major, C.GSS_C_GSS_CODE, nil, &mc, &mj)
	if major == C.GSS_S_COMPLETE && mj.value != nil && mj.length > 0 {
		str = string(C.GoBytes(mj.value, C.int(mj.length)))
		C.gss_release_buffer(nil, &mj)
	}
	if minor != C.GSS_S_COMPLETE {
		major = C.gss_display_status(&cm, minor, C.GSS_C_MECH_CODE, mech, &mc, &mn)
		if major == C.GSS_S_COMPLETE && mn.value != nil && mn.length > 0 {
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

func (n *negotiator) GetChallenge(w http.ResponseWriter, r *http.Request) error {
	w.Header().Add("WWW-Authenticate", "Negotiate")
	return nil
}

func (n *negotiator) CheckResponse(w http.ResponseWriter, r *http.Request) (User, error) {
	var ctx, tmpctx C.gss_ctx_id_t
	var creds C.gss_cred_id_t
	var itoken, otoken, namebuf, ictxtoken, octxtoken C.gss_buffer_desc
	var minor, lifetime, flags, exportable C.OM_uint32
	var name C.gss_name_t
	var mech C.gss_OID
	var client string
	var reply []byte
	var session *sessions.Session
	var open, local C.int

	ah := r.Header["Authorization"]
	for _, h := range ah {
		fields := strings.SplitN(strings.Replace(h, "\t", " ", -1), " ", 2)
		if strings.ToLower(fields[0]) == "negotiate" {
			// Decode the token we received from the client.
			logrus.Debugf("gssapi: got token \"%s\"", fields[1])
			token, err := base64.StdEncoding.DecodeString(fields[1])
			if err != nil {
				logrus.Errorf("error decoding Negotiate token: \"%s\"", fields[1])
				return User{}, err
			}
			// Check that we have credentials we can use to accept the client's initiator token.
			major := C.gss_acquire_cred(&minor, nil, C.GSS_C_INDEFINITE, nil, C.GSS_C_ACCEPT, &creds, nil, nil)
			if major != C.GSS_S_COMPLETE {
				logrus.Infof("error acquiring GSSAPI acceptor creds (%s), not accepting Negotiate auth", getErrorDesc(major, minor, nil))
				return User{}, nil
			}
			defer C.gss_release_cred(nil, &creds)
			// Limit the SPNEGO mechs we'll accept to plain Kerberos.  This lets us avoid looping if the client decides it needs to use IAKERB, like MIT clients did between 1.9 and when RT#8021 was merged.
			major = C.gss_set_neg_mechs(&minor, creds, &C.ServerSpnegoOidSet)
			if major != C.GSS_S_COMPLETE {
				logrus.Infof("error setting list of allowed GSSAPI mechanisms (%s), not accepting Negotiate auth", getErrorDesc(major, minor, nil))
				return User{}, nil
			}
			if cookies != nil {
				session, _ = cookies.Get(r, "docker")
				if session != nil {
					// Read a partially-established context.
					if ctxtoken, ok := session.Values[negotiator{}].([]byte); ok && len(ctxtoken) > 0 {
						ictxtoken.value = unsafe.Pointer(&ctxtoken[0])
						ictxtoken.length = C.size_t(len(ctxtoken))
						major := C.gss_import_sec_context(&minor, &ictxtoken, &tmpctx)
						if major != C.GSS_S_COMPLETE {
							logrus.Debug("error importing context token, ignoring")
						} else {
							ctx = tmpctx
						}
					}
					// Ignore a fully-established context, since we only get here if the client's still
					// trying to establish one.
					if ctx != nil {
						major = C.gss_inquire_context(&minor, ctx, &name, nil, &lifetime, nil, &flags, &local, &open)
						if major == C.GSS_S_COMPLETE && open != 0 {
							logrus.Debug("client provided established GSSAPI context, ignoring it")
							C.gss_delete_sec_context(&minor, &ctx, nil)
						}
					}
				}
			}
			// Actually accept the client's initiator token.
			itoken.value = unsafe.Pointer(&token[0])
			itoken.length = C.size_t(len(token))
			major = C.gss_accept_sec_context(&minor, &ctx, creds, &itoken, nil, &name, &mech, &otoken, &flags, &lifetime, nil)
			if otoken.length > 0 {
				reply = C.GoBytes(otoken.value, C.int(otoken.length))
				C.gss_release_buffer(&minor, &otoken)
			}
			// Check for failure.
			if major != C.GSS_S_COMPLETE && major != C.GSS_S_CONTINUE_NEEDED {
				logrus.Errorf("error accepting GSSAPI context (%s), failed Negotiate auth", getErrorDesc(major, minor, mech))
				return User{}, nil
			}
			if major == C.GSS_S_COMPLETE {
				logrus.Debug("accepted GSSAPI context")
				// Convert the GSSAPI name to one that can be displayed.
				major = C.gss_display_name(&minor, name, &namebuf, &mech)
				if major != C.GSS_S_COMPLETE {
					logrus.Errorf("error converting GSSAPI client name to display name (%s), failing Negotiate auth", getErrorDesc(major, minor, mech))
					return User{}, nil
				}
				client = string(C.GoBytes(namebuf.value, C.int(namebuf.length)))
				C.gss_release_buffer(&minor, &namebuf)
				exportable = C.GSS_C_TRANS_FLAG
			} else {
				logrus.Debug("more GSSAPI data required")
				exportable = C.GSS_C_PROT_READY_FLAG
			}
			// Save the established content in the client's session if we can, else drop it.
			if ctx != nil && session != nil && flags&exportable != 0 {
				major := C.gss_export_sec_context(&minor, &ctx, &octxtoken)
				if major != C.GSS_S_COMPLETE {
					logrus.Info("error exporting context token")
					defer C.gss_delete_sec_context(&minor, &ctx, nil)
				} else {
					logrus.Debug("exported GSSAPI context token")
					session.Values[negotiator{}] = C.GoBytes(octxtoken.value, C.int(octxtoken.length))
				}
			} else {
				logrus.Debug("can't export context token")
				defer C.gss_delete_sec_context(&minor, &ctx, nil)
			}
			// Format our reply token, if we have one, for the client.
			if len(reply) > 0 {
				token := base64.StdEncoding.EncodeToString(reply)
				logrus.Debugf("gssapi: produced reply token \"%s\"", token)
				w.Header().Add("WWW-Authenticate", "Negotiate "+token)
			}
			return User{Name: client, Scheme: n.Scheme}, nil
		}
	}
	return User{}, nil
}

func createNegotiate(options map[string]string) Authenticator {
	var creds C.gss_cred_id_t
	var minor C.OM_uint32

	keytab, ok := options["keytab"]
	if ok && keytab != "" {
		ckeytab := C.CString(keytab)
		defer C.free(unsafe.Pointer(ckeytab))
		major := C.gsskrb5_register_acceptor_identity(ckeytab)
		if major != C.GSS_S_COMPLETE {
			logrus.Errorf("error registering keytab \"%s\": %s", keytab, getErrorDesc(major, 0, nil))
			return nil
		}
	}
	// Check that we have credentials we can use to accept the client's initiator token.  If not, then don't offer Negotiate.
	major := C.gss_acquire_cred(&minor, nil, C.GSS_C_INDEFINITE, nil, C.GSS_C_ACCEPT, &creds, nil, nil)
	if major != C.GSS_S_COMPLETE {
		logrus.Debugf("unable to acquire GSSAPI acceptor creds (%s), not offering Negotiate auth", getErrorDesc(major, minor, nil))
		return nil
	}
	defer C.gss_release_cred(nil, &creds)
	return &negotiator{Scheme: "Negotiate", Name: "gssapi"}
}

func validateKeytabOption(option string) (string, error) {
	if strings.HasPrefix(option, "keytab=") {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	RegisterAuthenticator(createNegotiate)
	opts.RegisterAuthnOptionValidater(validateKeytabOption)
	gob.Register(negotiator{})
}
