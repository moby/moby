[![Build Status](https://travis-ci.org/miekg/dns.svg?branch=master)](https://travis-ci.org/miekg/dns)
[![Code Coverage](https://img.shields.io/codecov/c/github/miekg/dns/master.svg)](https://codecov.io/github/miekg/dns?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/miekg/dns)](https://goreportcard.com/report/miekg/dns)
[![](https://godoc.org/github.com/miekg/dns?status.svg)](https://godoc.org/github.com/miekg/dns)

# Alternative (more granular) approach to a DNS library

> Less is more.

Complete and usable DNS library. All Resource Records are supported, including the DNSSEC types.
It follows a lean and mean philosophy. If there is stuff you should know as a DNS programmer there
isn't a convenience function for it. Server side and client side programming is supported, i.e. you
can build servers and resolvers with it.

We try to keep the "master" branch as sane as possible and at the bleeding edge of standards,
avoiding breaking changes wherever reasonable. We support the last two versions of Go.

# Goals

* KISS;
* Fast;
* Small API. If it's easy to code in Go, don't make a function for it.

# Users

A not-so-up-to-date-list-that-may-be-actually-current:

* https://github.com/coredns/coredns
* https://cloudflare.com
* https://github.com/abh/geodns
* http://www.statdns.com/
* http://www.dnsinspect.com/
* https://github.com/chuangbo/jianbing-dictionary-dns
* http://www.dns-lg.com/
* https://github.com/fcambus/rrda
* https://github.com/kenshinx/godns
* https://github.com/skynetservices/skydns
* https://github.com/hashicorp/consul
* https://github.com/DevelopersPL/godnsagent
* https://github.com/duedil-ltd/discodns
* https://github.com/StalkR/dns-reverse-proxy
* https://github.com/tianon/rawdns
* https://mesosphere.github.io/mesos-dns/
* https://pulse.turbobytes.com/
* https://github.com/fcambus/statzone
* https://github.com/benschw/dns-clb-go
* https://github.com/corny/dnscheck for <http://public-dns.info/>
* https://namesmith.io
* https://github.com/miekg/unbound
* https://github.com/miekg/exdns
* https://dnslookup.org
* https://github.com/looterz/grimd
* https://github.com/phamhongviet/serf-dns
* https://github.com/mehrdadrad/mylg
* https://github.com/bamarni/dockness
* https://github.com/fffaraz/microdns
* http://kelda.io
* https://github.com/ipdcode/hades <https://jd.com>
* https://github.com/StackExchange/dnscontrol/
* https://www.dnsperf.com/
* https://dnssectest.net/
* https://dns.apebits.com
* https://github.com/oif/apex
* https://github.com/jedisct1/dnscrypt-proxy
* https://github.com/jedisct1/rpdns
* https://github.com/xor-gate/sshfp
* https://github.com/rs/dnstrace
* https://blitiri.com.ar/p/dnss ([github mirror](https://github.com/albertito/dnss))
* https://github.com/semihalev/sdns
* https://render.com
* https://github.com/peterzen/goresolver
* https://github.com/folbricht/routedns

Send pull request if you want to be listed here.

# Features

* UDP/TCP queries, IPv4 and IPv6
* RFC 1035 zone file parsing ($INCLUDE, $ORIGIN, $TTL and $GENERATE (for all record types) are supported
* Fast
* Server side programming (mimicking the net/http package)
* Client side programming
* DNSSEC: signing, validating and key generation for DSA, RSA, ECDSA and Ed25519
* EDNS0, NSID, Cookies
* AXFR/IXFR
* TSIG, SIG(0)
* DNS over TLS (DoT): encrypted connection between client and server over TCP
* DNS name compression

Have fun!

Miek Gieben  -  2010-2012  -  <miek@miek.nl>
DNS Authors 2012-

# Building

This library uses Go modules and uses semantic versioning. Building is done with the `go` tool, so
the following should work:

    go get github.com/miekg/dns
    go build github.com/miekg/dns

## Examples

A short "how to use the API" is at the beginning of doc.go (this also will show when you call `godoc
github.com/miekg/dns`).

Example programs can be found in the `github.com/miekg/exdns` repository.

## Supported RFCs

*all of them*

* 103{4,5} - DNS standard
* 1348 - NSAP record (removed the record)
* 1982 - Serial Arithmetic
* 1876 - LOC record
* 1995 - IXFR
* 1996 - DNS notify
* 2136 - DNS Update (dynamic updates)
* 2181 - RRset definition - there is no RRset type though, just []RR
* 2537 - RSAMD5 DNS keys
* 2065 - DNSSEC (updated in later RFCs)
* 2671 - EDNS record
* 2782 - SRV record
* 2845 - TSIG record
* 2915 - NAPTR record
* 2929 - DNS IANA Considerations
* 3110 - RSASHA1 DNS keys
* 3123 - APL record
* 3225 - DO bit (DNSSEC OK)
* 340{1,2,3} - NAPTR record
* 3445 - Limiting the scope of (DNS)KEY
* 3597 - Unknown RRs
* 403{3,4,5} - DNSSEC + validation functions
* 4255 - SSHFP record
* 4343 - Case insensitivity
* 4408 - SPF record
* 4509 - SHA256 Hash in DS
* 4592 - Wildcards in the DNS
* 4635 - HMAC SHA TSIG
* 4701 - DHCID
* 4892 - id.server
* 5001 - NSID
* 5155 - NSEC3 record
* 5205 - HIP record
* 5702 - SHA2 in the DNS
* 5936 - AXFR
* 5966 - TCP implementation recommendations
* 6605 - ECDSA
* 6725 - IANA Registry Update
* 6742 - ILNP DNS
* 6840 - Clarifications and Implementation Notes for DNS Security
* 6844 - CAA record
* 6891 - EDNS0 update
* 6895 - DNS IANA considerations
* 6944 - DNSSEC DNSKEY Algorithm Status
* 6975 - Algorithm Understanding in DNSSEC
* 7043 - EUI48/EUI64 records
* 7314 - DNS (EDNS) EXPIRE Option
* 7477 - CSYNC RR
* 7828 - edns-tcp-keepalive EDNS0 Option
* 7553 - URI record
* 7858 - DNS over TLS: Initiation and Performance Considerations
* 7871 - EDNS0 Client Subnet
* 7873 - Domain Name System (DNS) Cookies
* 8080 - EdDSA for DNSSEC
* 8499 - DNS Terminology

## Loosely Based Upon

* ldns - <https://nlnetlabs.nl/projects/ldns/about/>
* NSD - <https://nlnetlabs.nl/projects/nsd/about/>
* Net::DNS - <http://www.net-dns.org/>
* GRONG - <https://github.com/bortzmeyer/grong>
