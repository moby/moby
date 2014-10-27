package dnsserver

import (
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
)

// 53 could be in use by a local cache or w/e. 5353 is in use by avahi mDNS on
// my machine. Basically I don't want to have to write a port allocator.
const service = "127.0.0.1:5300"

var server = NewDNSServer("docker")

func init() {
	go func() {
		err := server.Listen(service)
		if err != nil {
			panic(err)
		}
	}()
}

func msgClient(fqdn string, dnsType uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(fqdn, dnsType)
	return dns.Exchange(m, service)
}

func TestARecordCRUD(t *testing.T) {
	table := map[string]net.IP{
		"test":  net.ParseIP("127.0.0.2"),
		"test2": net.ParseIP("127.0.0.3"),
	}

	// do this in independent parts so both records exist. This tests some
	// collision issues.
	for host, ip := range table {
		server.SetA(host, ip)
	}

	for host, ip := range table {
		msg, err := msgClient(fmt.Sprintf("%s.docker.", host), dns.TypeA)

		if err != nil {
			t.Fatal(err)
		}

		if len(msg.Answer) != 1 {
			t.Fatalf("Server did not reply with a valid answer.")
		}

		if msg.Answer[0].Header().Ttl != 1 {
			t.Fatalf("TTL was %d instead of 1", msg.Answer[0].Header().Ttl)
		}

		if msg.Answer[0].Header().Rrtype != dns.TypeA {
			t.Fatalf("Expected A record, got a record of type %d instead.", msg.Answer[0].Header().Rrtype)
		}

		if msg.Answer[0].Header().Name != fmt.Sprintf("%s.docker.", host) {
			// If this fails, we probably need to look at miekg/dns as this should
			// not be possible.
			t.Fatalf("Name does not match query sent, %q was provided", msg.Answer[0].Header().Name)
		}

		aRecord := msg.Answer[0].(*dns.A).A
		if !aRecord.Equal(ip) {
			t.Fatalf("IP %q does not match registered IP %q", aRecord, ip)
		}
	}

	for host := range table {
		server.DeleteA(host)
	}

	for host := range table {
		msg, err := msgClient(fmt.Sprintf("%s.docker.", host), dns.TypeA)

		if err != nil {
			t.Fatal(err)
		}

		if len(msg.Answer) != 0 {
			t.Fatal("Server gave a reply after record has been deleted")
		}
	}
}

func TestSRVRecordCRUD(t *testing.T) {
	table := map[string][]SRVRecord{
		"test":  {{80, "test.docker."}},
		"test2": {{81, "test2.docker."}},
	}

	// do this in independent parts so both records exist. This tests some
	// collision issues.
	for name, srv := range table {
		server.SetSRV(name, "tcp", srv)
	}

	for name, srv := range table {
		msg, err := msgClient(fmt.Sprintf("_%s._tcp.docker.", name), dns.TypeSRV)

		if err != nil {
			t.Fatal(err)
		}

		if len(msg.Answer) != 1 {
			t.Fatalf("Server did not reply with a valid answer.")
		}

		if msg.Answer[0].Header().Ttl != 1 {
			t.Fatalf("TTL was %d instead of 1", msg.Answer[0].Header().Ttl)
		}

		if msg.Answer[0].Header().Rrtype != dns.TypeSRV {
			t.Fatalf("Expected SRV record, got a record of type %d instead.", msg.Answer[0].Header().Rrtype)
		}

		if msg.Answer[0].Header().Name != fmt.Sprintf("_%s._tcp.docker.", name) {
			// If this fails, we probably need to look at miekg/dns as this should
			// not be possible.
			t.Fatalf("Name does not match query sent, %q was provided", msg.Answer[0].Header().Name)
		}

		srvRecord := msg.Answer[0].(*dns.SRV)

		if srvRecord.Priority != 0 || srvRecord.Weight != 0 {
			t.Fatal("Defaults for priority and weight do not equal 0")
		}

		result := SRVRecord{srvRecord.Port, srvRecord.Target}
		if !result.Equal(srv[0]) {
			t.Fatalf("SRV records are not equivalent: received host %q port %d", srvRecord.Target, srvRecord.Port)
		}
	}

	for name := range table {
		server.DeleteSRV(name, "tcp")
	}

	for name := range table {
		msg, err := msgClient(fmt.Sprintf("_%s._tcp.docker.", name), dns.TypeSRV)

		if err != nil {
			t.Fatal(err)
		}

		if len(msg.Answer) != 0 {
			t.Fatal("Server gave a reply after record has been deleted")
		}
	}
}
