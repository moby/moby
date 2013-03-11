package main

import (
	"../client"
	"flag"
	"log"
	"os"
	"path"
)

func main() {
	if cmd := path.Base(os.Args[0]); cmd == "docker" {
		fl_shell := flag.Bool("i", false, "Interactive mode")
		flag.Parse()
		if *fl_shell {
			if err := client.InteractiveMode(flag.Args()...); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := client.SimpleMode(os.Args[1:]); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		if err := client.SimpleMode(append([]string{cmd}, os.Args[1:]...)); err != nil {
			log.Fatal(err)
		}
	}
}
