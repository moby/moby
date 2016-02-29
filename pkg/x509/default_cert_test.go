package x509

import (
	"os"
	"testing"
)

func TestGenerateDefaultCA(t *testing.T) {
	caName, keyName := "default_ca.pem", "default_key.pem"
	if _, err := os.Stat(caName); !os.IsNotExist(err) {
		os.Remove(caName)
	}
	if _, err := os.Stat(keyName); !os.IsNotExist(err) {
		os.Remove(keyName)
	}
	err := GenerateDefaultCA(caName, keyName)
	if err != nil {
		t.Fatalf("Failed to generate default CA: caName: %s, keyName: %s; %v\n", caName, keyName, err)
	}
	os.Remove(caName)
	os.Remove(keyName)
}

func TestGenerateDefaultKeys(t *testing.T) {
	caName, caKeyName := "default_ca.pem", "default_ca_key.pem"
	certName, keyName := "default_cert.pem", "default_key.pem"
	if _, err := os.Stat(caName); !os.IsNotExist(err) {
		os.Remove(caName)
	}
	if _, err := os.Stat(caKeyName); !os.IsNotExist(err) {
		os.Remove(caKeyName)
	}
	if _, err := os.Stat(certName); !os.IsNotExist(err) {
		os.Remove(certName)
	}
	if _, err := os.Stat(keyName); !os.IsNotExist(err) {
		os.Remove(keyName)
	}
	err := GenerateDefaultCA(caName, caKeyName)
	if err != nil {
		t.Fatalf("Failed to generate default CA: caName: %s, caKeyName: %s; %v\n", caName, caKeyName, err)
	}
	err = GenerateDefaultKeys(certName, keyName, caName)
	if err != nil {
		t.Fatalf("Failed to generate default keys from ca: certName: %s, keyName: %s, caName: %s; %v\n", certName, keyName, caName, err)
	}
	os.Remove(caName)
	os.Remove(caKeyName)
	os.Remove(certName)
	os.Remove(keyName)
}
