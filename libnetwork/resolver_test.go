package libnetwork

import (
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/testutils"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/skip"
)

// a simple/null address type that will be used to fake a local address for unit testing
type tstaddr struct {
}

func (a *tstaddr) Network() string { return "tcp" }

func (a *tstaddr) String() string { return "127.0.0.1" }

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
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	defer testutils.SetupTestOSContext(t)()
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
	r := NewResolver(resolverIPSandbox, false, sb.(*sandbox))

	// test name1's IP is resolved correctly with the default A type query
	// Also make sure DNS lookups are case insensitive
	names := []string{"name1", "NaMe1"}
	for _, name := range names {
		q := new(dns.Msg)
		q.SetQuestion(name, dns.TypeA)
		r.(*resolver).ServeDNS(w, q)
		resp := w.GetResponse()
		checkNonNullResponse(t, resp)
		t.Log("Response: ", resp.String())
		checkDNSResponseCode(t, resp, dns.RcodeSuccess)
		checkDNSAnswersCount(t, resp, 1)
		checkDNSRRType(t, resp.Answer[0].Header().Rrtype, dns.TypeA)
		if answer, ok := resp.Answer[0].(*dns.A); ok {
			if !answer.A.Equal(net.ParseIP("192.168.0.1")) {
				t.Fatalf("IP response in Answer %v does not match 192.168.0.1", answer.A)
			}
		} else {
			t.Fatal("Answer of type A not found")
		}
		w.ClearResponse()
	}

	// test MX query with name1 results in Success response with 0 answer records
	q := new(dns.Msg)
	q.SetQuestion("name1", dns.TypeMX)
	r.(*resolver).ServeDNS(w, q)
	resp := w.GetResponse()
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

func newDNSHandlerServFailOnce(requests *int) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Compress = false
		if *requests == 0 {
			m.SetRcode(r, dns.RcodeServerFailure)
		}
		*requests = *requests + 1
		if err := w.WriteMsg(m); err != nil {
			logrus.WithError(err).Error("Error writing dns response")
		}
	}
}

func waitForLocalDNSServer(t *testing.T) {
	retries := 0
	maxRetries := 10

	for retries < maxRetries {
		t.Log("Try connecting to DNS server ...")
		// this test and retry mechanism only works for TCP. With UDP there is no
		// connection and the test becomes inaccurate leading to unpredictable results
		tconn, err := net.DialTimeout("tcp", "127.0.0.1:53", 10*time.Second)
		retries = retries + 1
		if err != nil {
			if oerr, ok := err.(*net.OpError); ok {
				// server is probably initializing
				if oerr.Err == syscall.ECONNREFUSED {
					continue
				}
			} else {
				// something is wrong: we should stop for analysis
				t.Fatal(err)
			}
		}
		if tconn != nil {
			tconn.Close()
			break
		}
	}
}

func TestDNSProxyServFail(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	osctx := testutils.SetupTestOSContextEx(t)
	defer osctx.Cleanup(t)

	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	n, err := c.NewNetwork("bridge", "dtnet2", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	sb, err := c.NewSandbox("c1")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	var nRequests int
	// initialize a local DNS server and configure it to fail the first query
	dns.HandleFunc(".", newDNSHandlerServFailOnce(&nRequests))
	// use TCP for predictable results. Connection tests (to figure out DNS server initialization) don't work with UDP
	server := &dns.Server{Addr: "127.0.0.1:53", Net: "tcp"}
	srvErrCh := make(chan error, 1)
	osctx.Go(t, func() {
		srvErrCh <- server.ListenAndServe()
	})
	defer func() {
		server.Shutdown() //nolint:errcheck
		if err := <-srvErrCh; err != nil {
			t.Error(err)
		}
	}()

	waitForLocalDNSServer(t)
	t.Log("DNS Server can be reached")

	w := new(tstwriter)
	r := NewResolver(resolverIPSandbox, true, sb.(*sandbox))
	q := new(dns.Msg)
	q.SetQuestion("name1.", dns.TypeA)

	var localDNSEntries []extDNSEntry
	extTestDNSEntry := extDNSEntry{IPStr: "127.0.0.1", HostLoopback: true}

	// configure two external DNS entries and point both to local DNS server thread
	localDNSEntries = append(localDNSEntries, extTestDNSEntry)
	localDNSEntries = append(localDNSEntries, extTestDNSEntry)

	// this should generate two requests: the first will fail leading to a retry
	r.(*resolver).SetExtServers(localDNSEntries)
	r.(*resolver).ServeDNS(w, q)
	if nRequests != 2 {
		t.Fatalf("Expected 2 DNS querries. Found: %d", nRequests)
	}
	t.Logf("Expected number of DNS requests generated")
}
