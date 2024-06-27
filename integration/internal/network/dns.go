package network

import (
	"net"
	"os"
	"testing"

	"github.com/miekg/dns"
	"gotest.tools/v3/assert"
)

const DNSRespAddr = "10.11.12.13"

// WriteTempResolvConf writes a resolv.conf that only contains a single
// nameserver line, with address addr.
// It returns the name of the temp file. The temp file will be deleted
// automatically by a t.Cleanup().
func WriteTempResolvConf(t *testing.T, addr string) string {
	t.Helper()
	// Not using t.TempDir() here because in rootless mode, while the temporary
	// directory gets mode 0777, it's a subdir of an 0700 directory owned by root.
	// So, it's not accessible by the daemon.
	f, err := os.CreateTemp("", "resolv.conf")
	assert.NilError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	err = f.Chmod(0o644)
	assert.NilError(t, err)
	f.Write([]byte("nameserver " + addr + "\n"))
	return f.Name()
}

// StartDaftDNS starts and returns a really, really daft DNS server that only
// responds to type-A requests, and always with address dnsRespAddr.
// The DNS server will be stopped automatically by a t.Cleanup().
func StartDaftDNS(t *testing.T, addr string) {
	serveDNS := func(w dns.ResponseWriter, query *dns.Msg) {
		if query.Question[0].Qtype == dns.TypeA {
			resp := &dns.Msg{}
			resp.SetReply(query)
			answer := &dns.A{
				Hdr: dns.RR_Header{
					Name:   query.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    600,
				},
			}
			answer.A = net.ParseIP(DNSRespAddr)
			resp.Answer = append(resp.Answer, answer)
			_ = w.WriteMsg(resp)
		}
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: 53,
	})
	assert.NilError(t, err)

	server := &dns.Server{Handler: dns.HandlerFunc(serveDNS), PacketConn: conn}
	go func() {
		_ = server.ActivateAndServe()
	}()

	t.Cleanup(func() { server.Shutdown() })
}
