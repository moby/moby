package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/flag"
)

var (
	i    int
	str  string
	b, h bool
)

func init() {
	flag.BoolVar(&b, []string{"b"}, false, "a simple bool")
	flag.IntVar(&i, []string{"#integer", "-integer"}, -1, "a simple integer")
	flag.StringVar(&str, []string{"s", "#hidden", "-string"}, "", "a simple string") //-s -hidden and --string will work, but -hidden won't be in the usage
	flag.BoolVar(&h, []string{"h", "#help", "-help"}, false, "display the help")
	flag.Parse()
}
func main() {
	if h {
		flag.PrintDefaults()
	}
	fmt.Printf("%s\n", str)
	fmt.Printf("%s\n", flag.Lookup("s").Value.String())
}
