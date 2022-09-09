# go-pkcs11uri

Welcome to the go-pkcs11uri library. The implementation follows [RFC 7512](https://tools.ietf.org/html/rfc7512) and this [errata](https://www.rfc-editor.org/errata/rfc7512).

# Exampe usage:

The following example builds on this library [here](https://github.com/miekg/pkcs11) and are using softhsm2 on Fedora.

## Example

This example program extending the one found [here](https://github.com/miekg/pkcs11/blob/master/README.md#examples):

```
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/miekg/pkcs11"
	pkcs11uri "github.com/stefanberger/go-pkcs11uri"
)

func main() {
	if len(os.Args) < 2 {
		panic("Missing pkcs11 URI argument")
	}
	uristr := os.Args[1]

	uri, err := pkcs11uri.New()
	if err != nil {
		panic(err)
	}
	err = uri.Parse(uristr)
	if err != nil {
		panic(err)
	}

	module, err := uri.GetModule()
	if err != nil {
		panic(err)
	}

	slot, ok := uri.GetPathAttribute("slot-id", false)
	if !ok {
		panic("No slot-id in pkcs11 URI")
	}
	slotid, err := strconv.Atoi(slot)
	if err != nil {
		panic(err)
	}

	pin, err := uri.GetPIN()
	if err != nil {
		panic(err)
	}

	p := pkcs11.New(module)
	err = p.Initialize()
	if err != nil {
		panic(err)
	}

	defer p.Destroy()
	defer p.Finalize()

	session, err := p.OpenSession(uint(slotid), pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		panic(err)
	}
	defer p.CloseSession(session)

	err = p.Login(session, pkcs11.CKU_USER, pin)
	if err != nil {
		panic(err)
	}
	defer p.Logout(session)

	p.DigestInit(session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_SHA_1, nil)})
	hash, err := p.Digest(session, []byte("this is a string"))
	if err != nil {
		panic(err)
	}

	for _, d := range hash {
		fmt.Printf("%x", d)
	}
	fmt.Println()
}
```

## Exampe Usage

```
$ sudo softhsm2-util --init-token --slot 1 --label test --pin 1234 --so-pin 1234
The token has been initialized and is reassigned to slot 2053753261
$ go build ./...
$ sudo ./pkcs11-example 'pkcs11:slot-id=2053753261?module-path=/usr/lib64/pkcs11/libsofthsm2.so&pin-value=1234'
517592df8fec3ad146a79a9af153db2a4d784ec5
```

