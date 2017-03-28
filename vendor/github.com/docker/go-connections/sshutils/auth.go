package sshutils

import (
	"path/filepath"

	"golang.org/x/crypto/ssh"

	"github.com/Sirupsen/logrus"
)

type config struct {
	identityFiles map[string][]byte // key: file path, value: content
	sshAuthSock   string            // socket file path
}

type envGetter func(string) string
type fileReader func(string) ([]byte, error)

func resolveAuthMethods(home string, envG envGetter, fileR fileReader) ([]ssh.AuthMethod, error) {
	c, err := resolveConfig(home, envG, fileR)
	if err != nil {
		return nil, err
	}

	var methods []ssh.AuthMethod
	if method := resolveAgent(c); method != nil {
		methods = append(methods, method)
	}
	if method := resolvePublicKeys(c); method != nil {
		methods = append(methods, method)
	}
	return methods, nil
}

// resolveConfig resolves the config
// TODO: use ~/.ssh/config and /etc/ssh/ssh_config
func resolveConfig(home string, envG envGetter, fileR fileReader) (*config, error) {
	var c config
	dotSSH := filepath.Join(home, ".ssh")
	identityFileCandidates := []string{
		// see ssh_config(5)
		filepath.Join(dotSSH, "id_rsa"),
		filepath.Join(dotSSH, "id_ed25519"),
		filepath.Join(dotSSH, "id_ecdsa"),
		filepath.Join(dotSSH, "id_dsa"),
		filepath.Join(dotSSH, "identity"),
	}
	c.identityFiles = make(map[string][]byte, 0)
	for _, cand := range identityFileCandidates {
		b, err := fileR(cand)
		if err == nil {
			c.identityFiles[cand] = b
		}
		// we can safely ignore err
	}
	c.sshAuthSock = envG("SSH_AUTH_SOCK")
	return &c, nil
}

func resolvePublicKeys(c *config) ssh.AuthMethod {
	var signers []ssh.Signer
	for filePath, b := range c.identityFiles {
		signer, err := ssh.ParsePrivateKey(b)
		if err == nil {
			logrus.Debugf("detected private key %s", filePath)
			signers = append(signers, signer)
		} else {
			logrus.Warnf("could not read private key %s: %v",
				filePath, err)
		}
	}
	auth := ssh.PublicKeys(signers...)
	return auth
}
