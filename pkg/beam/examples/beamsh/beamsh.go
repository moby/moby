package main

import (
	"fmt"
	"os"
	"github.com/dotcloud/docker/pkg/dockerscript"
)

func main() {
	script, err := dockerscript.Parse(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%d commands:\n", len(script))
	for i, cmd := range script {
		fmt.Printf("%%%d: %s\n", i, cmd)
	}
}
