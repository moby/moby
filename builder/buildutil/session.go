package buildutil

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/moby/buildkit/session"
	"github.com/pkg/errors"
)

const clientSessionRemote = "client-session"

var nodeID = "nodeID"

func isSessionSupported(c client.APIClient) bool {
	ping, err := c.Ping(context.TODO())
	if err != nil {
		panic(fmt.Errorf("could not ping: %v", err))
	}
	return ping.Experimental && versions.GreaterThanOrEqualTo(c.ClientVersion(), "1.31")
}

func trySession(c client.APIClient, contextDir string) (*session.Session, error) {
	var s *session.Session
	if isSessionSupported(c) {
		sharedKey, err := getBuildSharedKey(contextDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get build shared key")
		}
		s, err = session.NewSession(context.Background(), filepath.Base(contextDir), sharedKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create session")
		}
	}
	return s, nil
}

func getBuildSharedKey(dir string) (string, error) {
	// build session is hash of build dir with node based randomness
	s := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", tryNodeIdentifier(), dir)))
	return hex.EncodeToString(s[:]), nil
}

func tryNodeIdentifier() string {
	if nodeID == "nodeID" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err == nil {
			nodeID = hex.EncodeToString(b)
		}
	}
	return nodeID
}
