package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cpuguy83/go-md2man/v2/md2man"
)

var (
	inFilePath  = flag.String("in", "", "Path to file to be processed (default: stdin)")
	outFilePath = flag.String("out", "", "Path to output processed file (default: stdout)")
)

func main() {
	var err error
	flag.Parse()

	inFile := os.Stdin
	if *inFilePath != "" {
		inFile, err = os.Open(*inFilePath)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	defer inFile.Close() // nolint: errcheck

	doc, err := ioutil.ReadAll(inFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	out := md2man.Render(doc)

	outFile := os.Stdout
	if *outFilePath != "" {
		outFile, err = os.Create(*outFilePath)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer outFile.Close() // nolint: errcheck
	}
	_, err = outFile.Write(out)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
