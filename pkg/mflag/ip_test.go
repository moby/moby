package mflag

import (
	"net"
	"testing"
)

func TestIP(t *testing.T) {
	var ip net.IP
	if err := (*IP)(&ip).Set("1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if !ip.Equal(net.ParseIP("1.2.3.4")) {
		t.Fatalf("%#v\n", ip)
	}
	if s := (*IP)(&ip).String(); s != "1.2.3.4" {
		t.Fatalf("%#v\n", s)
	}

}
