package memberlist

import (
	"bytes"
	"reflect"
	"testing"
)

func TestPKCS7(t *testing.T) {
	for i := 0; i <= 255; i++ {
		// Make a buffer of size i
		buf := []byte{}
		for j := 0; j < i; j++ {
			buf = append(buf, byte(i))
		}

		// Copy to bytes buffer
		inp := bytes.NewBuffer(nil)
		inp.Write(buf)

		// Pad this out
		pkcs7encode(inp, 0, 16)

		// Unpad
		dec := pkcs7decode(inp.Bytes(), 16)

		// Ensure equivilence
		if !reflect.DeepEqual(buf, dec) {
			t.Fatalf("mismatch: %v %v", buf, dec)
		}
	}

}

func TestEncryptDecrypt_V0(t *testing.T) {
	encryptDecryptVersioned(0, t)
}

func TestEncryptDecrypt_V1(t *testing.T) {
	encryptDecryptVersioned(1, t)
}

func encryptDecryptVersioned(vsn encryptionVersion, t *testing.T) {
	k1 := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	plaintext := []byte("this is a plain text message")
	extra := []byte("random data")

	var buf bytes.Buffer
	err := encryptPayload(vsn, k1, plaintext, extra, &buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expLen := encryptedLength(vsn, len(plaintext))
	if buf.Len() != expLen {
		t.Fatalf("output length is unexpected %d %d %d", len(plaintext), buf.Len(), expLen)
	}

	msg, err := decryptPayload([][]byte{k1}, buf.Bytes(), extra)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	cmp := bytes.Compare(msg, plaintext)
	if cmp != 0 {
		t.Errorf("len %d %v", len(msg), msg)
		t.Errorf("len %d %v", len(plaintext), plaintext)
		t.Fatalf("encrypt/decrypt failed! %d '%s' '%s'", cmp, msg, plaintext)
	}
}
