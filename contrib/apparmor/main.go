package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"text/template"
)

type profileData struct{}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("pass a filename to save the profile in.")
	}

	// parse the arg
	apparmorProfilePath := os.Args[1]

	// parse the template
	compiled, err := template.New("apparmor_profile").Parse(dockerProfileTemplate)
	if err != nil {
		log.Fatalf("parsing template failed: %v", err)
	}

	// make sure /etc/apparmor.d exists
	if err := os.MkdirAll(path.Dir(apparmorProfilePath), 0755); err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile(apparmorProfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	data := profileData{}
	if err := compiled.Execute(f, data); err != nil {
		log.Fatalf("executing template failed: %v", err)
	}

	fmt.Printf("created apparmor profile for version %+v at %q\n", data, apparmorProfilePath)
}
