package logrus

import (
	"io/ioutil"
	"os/exec"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	current := len(handlers)
	RegisterExitHandler(func() {})
	if len(handlers) != current+1 {
		t.Fatalf("can't add handler")
	}
}

func TestHandler(t *testing.T) {
	gofile := "/tmp/testprog.go"
	if err := ioutil.WriteFile(gofile, testprog, 0666); err != nil {
		t.Fatalf("can't create go file")
	}

	outfile := "/tmp/testprog.out"
	arg := time.Now().UTC().String()
	err := exec.Command("go", "run", gofile, outfile, arg).Run()
	if err == nil {
		t.Fatalf("completed normally, should have failed")
	}

	data, err := ioutil.ReadFile(outfile)
	if err != nil {
		t.Fatalf("can't read output file %s", outfile)
	}

	if string(data) != arg {
		t.Fatalf("bad data")
	}
}

var testprog = []byte(`
// Test program for atexit, gets output file and data as arguments and writes
// data to output file in atexit handler.
package main

import (
	"github.com/sirupsen/logrus"
	"flag"
	"fmt"
	"io/ioutil"
)

var outfile = ""
var data = ""

func handler() {
	ioutil.WriteFile(outfile, []byte(data), 0666)
}

func badHandler() {
	n := 0
	fmt.Println(1/n)
}

func main() {
	flag.Parse()
	outfile = flag.Arg(0)
	data = flag.Arg(1)

	logrus.RegisterExitHandler(handler)
	logrus.RegisterExitHandler(badHandler)
	logrus.Fatal("Bye bye")
}
`)
