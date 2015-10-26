package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"
)

type profileData struct {
	MajorVersion int
	MinorVersion int
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("pass a filename to save the profile in.")
	}

	// parse the arg
	apparmorProfilePath := os.Args[1]

	// get the apparmor_version version
	cmd := exec.Command("/sbin/apparmor_parser", "--version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("getting apparmor_parser version failed: %s (%s)", err, output)
	}

	// parse the version from the output
	// output is in the form of the following:
	// AppArmor parser version 2.9.1
	// Copyright (C) 1999-2008 Novell Inc.
	// Copyright 2009-2012 Canonical Ltd.
	lines := strings.SplitN(string(output), "\n", 2)
	words := strings.Split(lines[0], " ")
	version := words[len(words)-1]
	// split by major minor version
	v := strings.Split(version, ".")
	if len(v) < 2 {
		log.Fatalf("parsing major minor version failed for %q", version)
	}

	majorVersion, err := strconv.Atoi(v[0])
	if err != nil {
		log.Fatal(err)
	}
	minorVersion, err := strconv.Atoi(v[1])
	if err != nil {
		log.Fatal(err)
	}
	data := profileData{
		MajorVersion: majorVersion,
		MinorVersion: minorVersion,
	}
	fmt.Printf("apparmor_parser is of version %+v\n", data)

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

	if err := compiled.Execute(f, data); err != nil {
		log.Fatalf("executing template failed: %v", err)
	}

	fmt.Printf("created apparmor profile for version %+v at %q\n", data, apparmorProfilePath)
}
