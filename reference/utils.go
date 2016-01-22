package reference

import (
	"fmt"
	"strings"

	distreference "github.com/docker/distribution/reference"
)

// SubstituteReferenceName creates a new image reference from given ref with
// its *name* part substituted for reposName.
func SubstituteReferenceName(ref Named, reposName string) (newRef Named, err error) {
	reposNameRef, err := WithName(reposName)
	if err != nil {
		return nil, err
	}
	if tagged, isTagged := ref.(distreference.Tagged); isTagged {
		newRef, err = WithTag(reposNameRef, tagged.Tag())
		if err != nil {
			return nil, err
		}
	} else if digested, isDigested := ref.(distreference.Digested); isDigested {
		newRef, err = WithDigest(reposNameRef, digested.Digest())
		if err != nil {
			return nil, err
		}
	} else {
		newRef = reposNameRef
	}
	return
}

// UnqualifyReference ...
func UnqualifyReference(ref Named) Named {
	_, remoteName, err := SplitReposName(ref)
	if err != nil {
		return ref
	}
	newRef, err := SubstituteReferenceName(ref, remoteName.Name())
	if err != nil {
		return ref
	}
	return newRef
}

// QualifyUnqualifiedReference ...
func QualifyUnqualifiedReference(ref Named, indexName string) (Named, error) {
	if !isValidHostname(indexName) {
		return nil, fmt.Errorf("Invalid hostname %q", indexName)
	}
	orig, remoteName, err := SplitReposName(ref)
	if err != nil {
		return nil, err
	}
	if orig == "" {
		return SubstituteReferenceName(ref, indexName+"/"+remoteName.Name())
	}
	return ref, nil
}

// IsReferenceFullyQualified determines whether the given reposName has prepended
// name of index.
func IsReferenceFullyQualified(reposName Named) bool {
	indexName, _, _ := SplitReposName(reposName)
	return indexName != ""
}

// SplitReposName breaks a reposName into an index name and remote name
func SplitReposName(reposName Named) (indexName string, remoteName Named, err error) {
	var remoteNameStr string
	indexName, remoteNameStr = distreference.SplitHostname(reposName)
	if !isValidHostname(indexName) {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = ""
		remoteName = reposName
	} else {
		remoteName, err = WithName(remoteNameStr)
	}
	return
}

func isValidHostname(hostname string) bool {
	return hostname != "" && !strings.Contains(hostname, "/") &&
		(strings.Contains(hostname, ".") ||
			strings.Contains(hostname, ":") || hostname == "localhost")
}
