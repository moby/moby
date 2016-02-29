package x509

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"time"
)

func GenerateDefaultCA(certPath, keyPath string) error {
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	return generateCert(certPath, keyPath, "", keyUsage, extKeyUsage, true)
}

func GenerateDefaultKeys(certPath, keyPath, caPath string) error {
	keyUsage := x509.KeyUsageDigitalSignature
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	return generateCert(certPath, keyPath, caPath, keyUsage, extKeyUsage, false)
}

func generateCert(certPath, keyPath, caPath string, keyUsage x509.KeyUsage, extKeyUsage []x509.ExtKeyUsage, isCA bool) error {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("Failed to generate CA key: %v\n", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(1, 0, 0)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("Failed to generate cert serial number: %v\n", err)
	}

	certTemplate := x509.Certificate{
		SignatureAlgorithm: x509.SHA512WithRSA,
		PublicKeyAlgorithm: x509.RSA,

		SerialNumber: serialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     keyUsage,
		ExtKeyUsage:  extKeyUsage,

		BasicConstraintsValid: true,
		IsCA: isCA,
	}

	var derBytes []byte
	if !isCA {
		caBytes, err := ioutil.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("Could not read CA Certificate: %v\n", err)
		}
		block, rest := pem.Decode(caBytes)
		if len(rest) != 0 || block.Type != "CERTIFICATE" {
			return fmt.Errorf("Failed to decode CA Certificate\n")
		}

		caCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("Could not parse certificate: %v\n", err)
		}
		derBytes, err = x509.CreateCertificate(rand.Reader, &certTemplate, caCert, &priv.PublicKey, priv)
	} else {
		derBytes, err = x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &priv.PublicKey, priv)
	}

	if err != nil {
		return fmt.Errorf("Could not create certificate: %v\n", err)
	}

	cert, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0444)
	if err != nil {
		return fmt.Errorf("Could not create file %s: %v\n", certPath, err)
	}
	pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	cert.Close()

	key, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return fmt.Errorf("Could not create file %s: %v\n", keyPath, err)
	}
	privDerBytes := x509.MarshalPKCS1PrivateKey(priv)
	pem.Encode(key, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDerBytes})
	key.Close()

	return nil
}
