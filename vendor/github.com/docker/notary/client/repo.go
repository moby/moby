// +build !pkcs11

package client

import (
	"fmt"
	"net/http"

	"github.com/docker/notary"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/trustpinning"
)

// NewNotaryRepository is a helper method that returns a new notary repository.
// It takes the base directory under where all the trust files will be stored
// (This is normally defaults to "~/.notary" or "~/.docker/trust" when enabling
// docker content trust).
func NewNotaryRepository(baseDir, gun, baseURL string, rt http.RoundTripper,
	retriever notary.PassRetriever, trustPinning trustpinning.TrustPinConfig) (
	*NotaryRepository, error) {

	fileKeyStore, err := trustmanager.NewKeyFileStore(baseDir, retriever)
	if err != nil {
		return nil, fmt.Errorf("failed to create private key store in directory: %s", baseDir)
	}

	return repositoryFromKeystores(baseDir, gun, baseURL, rt,
		[]trustmanager.KeyStore{fileKeyStore}, trustPinning)
}
