package main

import (
	"../client"
	"log"
	"os"
)

func main() {
	if err := client.SimpleMode(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
