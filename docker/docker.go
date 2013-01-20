package main

import (
	"io"
	"log"
	"os"
	"flag"
	"net/http"
	"net/url"
)


// Use this key to encode an RPC call into an URL,
// eg. domain.tld/path/to/method?q=get_user&q=gordon
const ARG_URL_KEY = "q"

func CallToURL(host string, cmd string, args []string) *url.URL {
    qValues := make(url.Values)
    for _, v := range args {
        qValues.Add(ARG_URL_KEY, v)
    }
    return &url.URL{
	Scheme:     "http",
	Host:       host,
        Path:       "/" + cmd,
        RawQuery:   qValues.Encode(),
    }
}


func main() {
	flag.Parse()
	u := CallToURL(os.Getenv("DOCKER"), flag.Args()[0], flag.Args()[1:])
	resp, err := http.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, resp.Body)
}
