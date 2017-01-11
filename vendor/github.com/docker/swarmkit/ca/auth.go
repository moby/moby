package ca

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type localRequestKeyType struct{}

// LocalRequestKey is a context key to mark a request that originating on the
// local node. The assocated value is a RemoteNodeInfo structure describing the
// local node.
var LocalRequestKey = localRequestKeyType{}

// LogTLSState logs information about the TLS connection and remote peers
func LogTLSState(ctx context.Context, tlsState *tls.ConnectionState) {
	if tlsState == nil {
		log.G(ctx).Debugf("no TLS Chains found")
		return
	}

	peerCerts := []string{}
	verifiedChain := []string{}
	for _, cert := range tlsState.PeerCertificates {
		peerCerts = append(peerCerts, cert.Subject.CommonName)
	}
	for _, chain := range tlsState.VerifiedChains {
		subjects := []string{}
		for _, cert := range chain {
			subjects = append(subjects, cert.Subject.CommonName)
		}
		verifiedChain = append(verifiedChain, strings.Join(subjects, ","))
	}

	log.G(ctx).WithFields(logrus.Fields{
		"peer.peerCert": peerCerts,
		// "peer.verifiedChain": verifiedChain},
	}).Debugf("")
}

// getCertificateSubject extracts the subject from a verified client certificate
func getCertificateSubject(tlsState *tls.ConnectionState) (pkix.Name, error) {
	if tlsState == nil {
		return pkix.Name{}, grpc.Errorf(codes.PermissionDenied, "request is not using TLS")
	}
	if len(tlsState.PeerCertificates) == 0 {
		return pkix.Name{}, grpc.Errorf(codes.PermissionDenied, "no client certificates in request")
	}
	if len(tlsState.VerifiedChains) == 0 {
		return pkix.Name{}, grpc.Errorf(codes.PermissionDenied, "no verified chains for remote certificate")
	}

	return tlsState.VerifiedChains[0][0].Subject, nil
}

func tlsConnStateFromContext(ctx context.Context) (*tls.ConnectionState, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, grpc.Errorf(codes.PermissionDenied, "Permission denied: no peer info")
	}
	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, grpc.Errorf(codes.PermissionDenied, "Permission denied: peer didn't not present valid peer certificate")
	}
	return &tlsInfo.State, nil
}

// certSubjectFromContext extracts pkix.Name from context.
func certSubjectFromContext(ctx context.Context) (pkix.Name, error) {
	connState, err := tlsConnStateFromContext(ctx)
	if err != nil {
		return pkix.Name{}, err
	}
	return getCertificateSubject(connState)
}

// AuthorizeOrgAndRole takes in a context and a list of roles, and returns
// the Node ID of the node.
func AuthorizeOrgAndRole(ctx context.Context, org string, blacklistedCerts map[string]*api.BlacklistedCertificate, ou ...string) (string, error) {
	certSubj, err := certSubjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	// Check if the current certificate has an OU that authorizes
	// access to this method
	if intersectArrays(certSubj.OrganizationalUnit, ou) {
		return authorizeOrg(certSubj, org, blacklistedCerts)
	}

	return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: remote certificate not part of OUs: %v", ou)
}

// authorizeOrg takes in a certificate subject and an organization, and returns
// the Node ID of the node.
func authorizeOrg(certSubj pkix.Name, org string, blacklistedCerts map[string]*api.BlacklistedCertificate) (string, error) {
	if _, ok := blacklistedCerts[certSubj.CommonName]; ok {
		return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: node %s was removed from swarm", certSubj.CommonName)
	}

	if len(certSubj.Organization) > 0 && certSubj.Organization[0] == org {
		return certSubj.CommonName, nil
	}

	return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: remote certificate not part of organization: %s", org)
}

// AuthorizeForwardedRoleAndOrg checks for proper roles and organization of caller. The RPC may have
// been proxied by a manager, in which case the manager is authenticated and
// so is the certificate information that it forwarded. It returns the node ID
// of the original client.
func AuthorizeForwardedRoleAndOrg(ctx context.Context, authorizedRoles, forwarderRoles []string, org string, blacklistedCerts map[string]*api.BlacklistedCertificate) (string, error) {
	if isForwardedRequest(ctx) {
		_, err := AuthorizeOrgAndRole(ctx, org, blacklistedCerts, forwarderRoles...)
		if err != nil {
			return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: unauthorized forwarder role: %v", err)
		}

		// This was a forwarded request. Authorize the forwarder, and
		// check if the forwarded role matches one of the authorized
		// roles.
		_, forwardedID, forwardedOrg, forwardedOUs := forwardedTLSInfoFromContext(ctx)

		if len(forwardedOUs) == 0 || forwardedID == "" || forwardedOrg == "" {
			return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: missing information in forwarded request")
		}

		if !intersectArrays(forwardedOUs, authorizedRoles) {
			return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: unauthorized forwarded role, expecting: %v", authorizedRoles)
		}

		if forwardedOrg != org {
			return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: organization mismatch, expecting: %s", org)
		}

		return forwardedID, nil
	}

	// There wasn't any node being forwarded, check if this is a direct call by the expected role
	nodeID, err := AuthorizeOrgAndRole(ctx, org, blacklistedCerts, authorizedRoles...)
	if err == nil {
		return nodeID, nil
	}

	return "", grpc.Errorf(codes.PermissionDenied, "Permission denied: unauthorized peer role: %v", err)
}

// intersectArrays returns true when there is at least one element in common
// between the two arrays
func intersectArrays(orig, tgt []string) bool {
	for _, i := range orig {
		for _, x := range tgt {
			if i == x {
				return true
			}
		}
	}
	return false
}

// RemoteNodeInfo describes a node sending an RPC request.
type RemoteNodeInfo struct {
	// Roles is a list of roles contained in the node's certificate
	// (or forwarded by a trusted node).
	Roles []string

	// Organization is the organization contained in the node's certificate
	// (or forwarded by a trusted node).
	Organization string

	// NodeID is the node's ID, from the CN field in its certificate
	// (or forwarded by a trusted node).
	NodeID string

	// ForwardedBy contains information for the node that forwarded this
	// request. It is set to nil if the request was received directly.
	ForwardedBy *RemoteNodeInfo

	// RemoteAddr is the address that this node is connecting to the cluster
	// from.
	RemoteAddr string
}

// RemoteNode returns the node ID and role from the client's TLS certificate.
// If the RPC was forwarded, the original client's ID and role is returned, as
// well as the forwarder's ID. This function does not do authorization checks -
// it only looks up the node ID.
func RemoteNode(ctx context.Context) (RemoteNodeInfo, error) {
	// If we have a value on the context that marks this as a local
	// request, we return the node info from the context.
	localNodeInfo := ctx.Value(LocalRequestKey)

	if localNodeInfo != nil {
		nodeInfo, ok := localNodeInfo.(RemoteNodeInfo)
		if ok {
			return nodeInfo, nil
		}
	}

	certSubj, err := certSubjectFromContext(ctx)
	if err != nil {
		return RemoteNodeInfo{}, err
	}

	org := ""
	if len(certSubj.Organization) > 0 {
		org = certSubj.Organization[0]
	}

	peer, ok := peer.FromContext(ctx)
	if !ok {
		return RemoteNodeInfo{}, grpc.Errorf(codes.PermissionDenied, "Permission denied: no peer info")
	}

	directInfo := RemoteNodeInfo{
		Roles:        certSubj.OrganizationalUnit,
		NodeID:       certSubj.CommonName,
		Organization: org,
		RemoteAddr:   peer.Addr.String(),
	}

	if isForwardedRequest(ctx) {
		remoteAddr, cn, org, ous := forwardedTLSInfoFromContext(ctx)
		if len(ous) == 0 || cn == "" || org == "" {
			return RemoteNodeInfo{}, grpc.Errorf(codes.PermissionDenied, "Permission denied: missing information in forwarded request")
		}
		return RemoteNodeInfo{
			Roles:        ous,
			NodeID:       cn,
			Organization: org,
			ForwardedBy:  &directInfo,
			RemoteAddr:   remoteAddr,
		}, nil
	}

	return directInfo, nil
}
