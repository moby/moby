package main

import (
	"fmt"
	flag "github.com/dotcloud/docker/pkg/mflag"
)

var (
	i        int
	str      string
	b, b2, h bool
)

func init() {
	flag.BoolVar(&b, []string{"b"}, false, "a simple bool")
	flag.BoolVar(&b2, []string{"-bool"}, false, "a simple bool")
	flag.IntVar(&i, []string{"#integer", "-integer"}, -1, "a simple integer")
	flag.StringVar(&str, []string{"s", "#hidden", "-string"}, "", "a simple string") //-s -hidden and --string will work, but -hidden won't be in the usage
	flag.BoolVar(&h, []string{"h", "#help", "-help"}, false, "display the help")
	flag.Parse()
}
func main() {
	if h {
		flag.PrintDefaults()
	}
	fmt.Printf("s/#hidden/-string: %s\n", str)
	fmt.Printf("b: %b\n", b)
	fmt.Printf("-bool: %b\n", b2)
	fmt.Printf("s/#hidden/-string(via lookup): %s\n", flag.Lookup("s").Value.String())
}
