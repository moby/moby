package etchosts

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestBuildHostnameDomainname(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "10.11.12.13", "testhostname", "testdomainname", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildHostname(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "10.11.12.13", "testhostname", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildNoIP(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "testhostname", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := ""; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}
