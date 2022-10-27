package authorization // import "github.com/docker/docker/pkg/authorization"

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPeerCertificateMarshalJSON(t *testing.T) {
	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3},
		SerialNumber:          big.NewInt(1234),
		Subject: pkix.Name{
			Country:      []string{"Earth"},
			Organization: []string{"Mother Nature"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(5, 5, 5),

		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	// generate private key
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NilError(t, err)
	publickey := &privatekey.PublicKey

	// create a self-signed certificate. template = parent
	var parent = template
	raw, err := x509.CreateCertificate(rand.Reader, template, parent, publickey, privatekey)
	assert.NilError(t, err)

	cert, err := x509.ParseCertificate(raw)
	assert.NilError(t, err)

	var certs = []*x509.Certificate{cert}
	addr := "www.authz.com/auth"
	req, err := http.NewRequest(http.MethodGet, addr, nil)
	assert.NilError(t, err)

	req.RequestURI = addr
	req.TLS = &tls.ConnectionState{}
	req.TLS.PeerCertificates = certs
	req.Header.Add("header", "value")

	for _, c := range req.TLS.PeerCertificates {
		pcObj := PeerCertificate(*c)

		t.Run("Marshalling :", func(t *testing.T) {
			raw, err = pcObj.MarshalJSON()
			assert.Assert(t, raw != nil)
			assert.NilError(t, err)
		})

		t.Run("UnMarshalling :", func(t *testing.T) {
			err := pcObj.UnmarshalJSON(raw)
			assert.Assert(t, is.Nil(err))
			assert.Equal(t, "Earth", pcObj.Subject.Country[0])
			assert.Equal(t, true, pcObj.IsCA)
		})
	}
}
