package auth

import "github.com/docker/docker/client/session"

func AuthProviderServiceName() string {
	return _AuthConfigProvider_serviceDesc.ServiceName
}

func AttachAuthConfigProviderToSession(p AuthConfigProviderServer, sess *session.ServerSession) {
	sess.Allow(&_AuthConfigProvider_serviceDesc, p)
}
