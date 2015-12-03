package etchosts

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

func TestBuildDefault(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// check that /etc/hosts has consistent ordering
	for i := 0; i <= 5; i++ {
		err = Build(file.Name(), "", "", "", nil)
		if err != nil {
			t.Fatal(err)
		}

		content, err := ioutil.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		expected := "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\nfe00::0\tip6-localnet\nff00::0\tip6-mcastprefix\nff02::1\tip6-allnodes\nff02::2\tip6-allrouters\n"

		if expected != string(content) {
			t.Fatalf("Expected to find '%s' got '%s'", expected, content)
		}
	}
}

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

func TestUpdate(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := Build(file.Name(), "10.11.12.13", "testhostname", "testdomainname", nil); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if err := Update(file.Name(), "1.1.1.1", "testhostname"); err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "1.1.1.1\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestAddEmpty(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		Record{
			Hosts: "testhostname",
			IP:    "2.2.2.2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestDeleteEmpty(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		Record{
			Hosts: "testhostname1",
			IP:    "1.1.1.1",
		},
		Record{
			Hosts: "testhostname2",
			IP:    "2.2.2.2",
		},
		Record{
			Hosts: "testhostname3",
			IP:    "3.3.3.3",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{
		Record{
			Hosts: "testhostname1",
			IP:    "1.1.1.1",
		},
		Record{
			Hosts: "testhostname3",
			IP:    "3.3.3.3",
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname2\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\ttesthostname1\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}

func TestConcurrentWrites(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		Record{
			Hosts: "inithostname",
			IP:    "172.17.0.1",
		},
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			rec := []Record{
				Record{
					IP:    fmt.Sprintf("%d.%d.%d.%d", i, i, i, i),
					Hosts: fmt.Sprintf("testhostname%d", i),
				},
			}

			for j := 0; j < 25; j++ {
				if err := Add(file.Name(), rec); err != nil {
					t.Fatal(err)
				}

				if err := Delete(file.Name(), rec); err != nil {
					t.Fatal(err)
				}
			}
		}()
	}

	wg.Wait()

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "172.17.0.1\tinithostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func benchDelete(b *testing.B) {
	b.StopTimer()
	file, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		b.StopTimer()
		file.Close()
		os.Remove(file.Name())
		b.StartTimer()
	}()

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		b.Fatal(err)
	}

	var records []Record
	var toDelete []Record
	for i := 0; i < 255; i++ {
		record := Record{
			Hosts: fmt.Sprintf("testhostname%d", i),
			IP:    fmt.Sprintf("%d.%d.%d.%d", i, i, i, i),
		}
		records = append(records, record)
		if i%2 == 0 {
			toDelete = append(records, record)
		}
	}

	if err := Add(file.Name(), records); err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	if err := Delete(file.Name(), toDelete); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkDelete(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchDelete(b)
	}
}
