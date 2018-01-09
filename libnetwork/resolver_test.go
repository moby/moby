package libnetwork

import (
	"bytes"
	"net"
	"testing"

	"github.com/miekg/dns"
)

// a simple/null address type that will be used to fake a local address for unit testing
type tstaddr struct {
	net string
}

func (a *tstaddr) Network() string { return "tcp" }

func (a *tstaddr) String() string { return "" }

// a simple writer that implements dns.ResponseWriter for unit testing purposes
type tstwriter struct {
	msg *dns.Msg
}

func (w *tstwriter) WriteMsg(m *dns.Msg) (err error) {
	w.msg = m
	return nil
}

func (w *tstwriter) Write(m []byte) (int, error) { return 0, nil }

func (w *tstwriter) LocalAddr() net.Addr { return new(tstaddr) }

func (w *tstwriter) RemoteAddr() net.Addr { return new(tstaddr) }

func (w *tstwriter) TsigStatus() error { return nil }

func (w *tstwriter) TsigTimersOnly(b bool) {}

func (w *tstwriter) Hijack() {}

func (w *tstwriter) Close() error { return nil }

func (w *tstwriter) GetResponse() *dns.Msg { return w.msg }

func (w *tstwriter) ClearResponse() { w.msg = nil }

func checkNonNullResponse(t *testing.T, m *dns.Msg) {
	if m == nil {
		t.Fatal("Null DNS response found. Non Null response msg expected.")
	}
}

func checkNullResponse(t *testing.T, m *dns.Msg) {
	if m != nil {
		t.Fatal("Non Null DNS response found. Null response msg expected.")
	}
}

func checkDNSAnswersCount(t *testing.T, m *dns.Msg, expected int) {
	answers := len(m.Answer)
	if answers != expected {
		t.Fatalf("Expected number of answers in response: %d. Found: %d", expected, answers)
	}
}

func checkDNSResponseCode(t *testing.T, m *dns.Msg, expected int) {
	if m.MsgHdr.Rcode != expected {
		t.Fatalf("Expected DNS response code: %d. Found: %d", expected, m.MsgHdr.Rcode)
	}
}

func checkDNSRRType(t *testing.T, actual, expected uint16) {
	if actual != expected {
		t.Fatalf("Expected DNS Rrtype: %d. Found: %d", expected, actual)
	}
}

func TestDNSIPQuery(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	n, err := c.NewNetwork("bridge", "dtnet1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	sb, err := c.NewSandbox("c1")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	// we need the endpoint only to populate ep_list for the sandbox as part of resolve_name
	// it is not set as a target for name resolution and does not serve any other purpose
	err = ep.Join(sb)
	if err != nil {
		t.Fatal(err)
	}

	// add service records which are used to resolve names. These are the real targets for the DNS querries
	n.(*network).addSvcRecords("ep1", "name1", "svc1", net.ParseIP("192.168.0.1"), net.IP{}, true, "test")

	w := new(tstwriter)
	// the unit tests right now will focus on non-proxyed DNS requests
	r := NewResolver(resolverIPSandbox, false, sb.Key(), sb.(*sandbox))

	// test name1's IP is resolved correctly with the default A type query
	q := new(dns.Msg)
	q.SetQuestion("name1", dns.TypeA)
	r.(*resolver).ServeDNS(w, q)
	resp := w.GetResponse()
	checkNonNullResponse(t, resp)
	t.Log("Response: ", resp.String())
	checkDNSResponseCode(t, resp, dns.RcodeSuccess)
	checkDNSAnswersCount(t, resp, 1)
	checkDNSRRType(t, resp.Answer[0].Header().Rrtype, dns.TypeA)
	if answer, ok := resp.Answer[0].(*dns.A); ok {
		if !bytes.Equal(answer.A, net.ParseIP("192.168.0.1")) {
			t.Fatalf("IP response in Answer %v does not match 192.168.0.1", answer.A)
		}
	} else {
		t.Fatal("Answer of type A not found")
	}
	w.ClearResponse()

	// test MX query with name1 results in Success response with 0 answer records
	q = new(dns.Msg)
	q.SetQuestion("name1", dns.TypeMX)
	r.(*resolver).ServeDNS(w, q)
	resp = w.GetResponse()
	checkNonNullResponse(t, resp)
	t.Log("Response: ", resp.String())
	checkDNSResponseCode(t, resp, dns.RcodeSuccess)
	checkDNSAnswersCount(t, resp, 0)
	w.ClearResponse()

	// test MX query with non existent name results in ServFail response with 0 answer records
	// since this is a unit test env, we disable proxying DNS above which results in ServFail rather than NXDOMAIN
	q = new(dns.Msg)
	q.SetQuestion("nonexistent", dns.TypeMX)
	r.(*resolver).ServeDNS(w, q)
	resp = w.GetResponse()
	checkNonNullResponse(t, resp)
	t.Log("Response: ", resp.String())
	checkDNSResponseCode(t, resp, dns.RcodeServerFailure)
	w.ClearResponse()

}
