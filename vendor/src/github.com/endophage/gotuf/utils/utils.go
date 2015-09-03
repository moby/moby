package utils

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/endophage/gotuf/data"
)

func Download(url url.URL) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client.Get(url.String())
}

func Upload(url string, body io.Reader) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client.Post(url, "application/json", body)
}

func ValidateTarget(r io.Reader, m *data.FileMeta) error {
	h := sha256.New()
	length, err := io.Copy(h, r)
	if err != nil {
		return err
	}
	if length != m.Length {
		return fmt.Errorf("Size of downloaded target did not match targets entry.\nExpected: %s\nReceived: %s\n", m.Length, length)
	}
	hashDigest := h.Sum(nil)
	if bytes.Compare(m.Hashes["sha256"], hashDigest[:]) != 0 {
		return fmt.Errorf("Hash of downloaded target did not match targets entry.\nExpected: %x\nReceived: %x\n", m.Hashes["sha256"], hashDigest)
	}
	return nil
}

func StrSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func StrSliceContainsI(ss []string, s string) bool {
	s = strings.ToLower(s)
	for _, v := range ss {
		v = strings.ToLower(v)
		if v == s {
			return true
		}
	}
	return false
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

type NoopCloser struct {
	io.Reader
}

func (nc *NoopCloser) Close() error {
	return nil
}

func DoHash(alg string, d []byte) []byte {
	switch alg {
	case "sha256":
		digest := sha256.Sum256(d)
		return digest[:]
	case "sha512":
		digest := sha512.Sum512(d)
		return digest[:]
	}
	return nil
}
