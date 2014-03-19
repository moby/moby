package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/reexec"
	"os"
)

func main() {
	fmt.Println(os.Args[0])

	if os.Args[0] == "./test" {
		cmd := reexec.Command("other")

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			panic(err)
		}
	}
}
