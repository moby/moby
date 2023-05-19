package libnetwork

import (
	"encoding/hex"
	"errors"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/testutils"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// a simple/null address type that will be used to fake a local address for unit testing
type tstaddr struct {
	network string
}

func (a *tstaddr) Network() string {
	if a.network != "" {
		return a.network
	}
	return "tcp"
}

func (a *tstaddr) String() string { return "(fake)" }

// a simple writer that implements dns.ResponseWriter for unit testing purposes
type tstwriter struct {
	network string
	msg     *dns.Msg
}

func (w *tstwriter) WriteMsg(m *dns.Msg) (err error) {
	w.msg = m
	return nil
}

func (w *tstwriter) Write(m []byte) (int, error) { return 0, nil }

func (w *tstwriter) LocalAddr() net.Addr {
	return &tstaddr{network: w.network}
}

func (w *tstwriter) RemoteAddr() net.Addr {
	return &tstaddr{network: w.network}
}

func (w *tstwriter) TsigStatus() error { return nil }

func (w *tstwriter) TsigTimersOnly(b bool) {}

func (w *tstwriter) Hijack() {}

func (w *tstwriter) Close() error { return nil }

func (w *tstwriter) GetResponse() *dns.Msg { return w.msg }

func (w *tstwriter) ClearResponse() { w.msg = nil }

func checkNonNullResponse(t *testing.T, m *dns.Msg) {
	t.Helper()
	if m == nil {
		t.Fatal("Null DNS response found. Non Null response msg expected.")
	}
}

func checkDNSAnswersCount(t *testing.T, m *dns.Msg, expected int) {
	t.Helper()
	answers := len(m.Answer)
	if answers != expected {
		t.Fatalf("Expected number of answers in response: %d. Found: %d", expected, answers)
	}
}

func checkDNSResponseCode(t *testing.T, m *dns.Msg, expected int) {
	t.Helper()
	if m.MsgHdr.Rcode != expected {
		t.Fatalf("Expected DNS response code: %d. Found: %d", expected, m.MsgHdr.Rcode)
	}
}

func checkDNSRRType(t *testing.T, actual, expected uint16) {
	t.Helper()
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
	r := NewResolver(resolverIPSandbox, false, sb)

	// test name1's IP is resolved correctly with the default A type query
	// Also make sure DNS lookups are case insensitive
	names := []string{"name1", "NaMe1"}
	for _, name := range names {
		q := new(dns.Msg)
		q.SetQuestion(name, dns.TypeA)
		r.serveDNS(w, q)
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
	r.serveDNS(w, q)
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
	r.serveDNS(w, q)
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
	r := NewResolver(resolverIPSandbox, true, sb)
	q := new(dns.Msg)
	q.SetQuestion("name1.", dns.TypeA)

	var localDNSEntries []extDNSEntry
	extTestDNSEntry := extDNSEntry{IPStr: "127.0.0.1", HostLoopback: true}

	// configure two external DNS entries and point both to local DNS server thread
	localDNSEntries = append(localDNSEntries, extTestDNSEntry)
	localDNSEntries = append(localDNSEntries, extTestDNSEntry)

	// this should generate two requests: the first will fail leading to a retry
	r.SetExtServers(localDNSEntries)
	r.serveDNS(w, q)
	if nRequests != 2 {
		t.Fatalf("Expected 2 DNS querries. Found: %d", nRequests)
	}
	t.Logf("Expected number of DNS requests generated")
}

// Packet 24 extracted from
// https://gist.github.com/vojtad/3bac63b8c91b1ec50e8d8b36047317fa/raw/7d75eb3d3448381bf252ae55ea5123a132c46658/host.pcap
// (https://github.com/moby/moby/issues/44575)
// which is a non-compliant DNS reply > 512B (w/o EDNS(0)) to the query
//
//	s3.amazonaws.com. IN A
const oversizedDNSReplyMsg = "\xf5\x11\x81\x80\x00\x01\x00\x20\x00\x00\x00\x00\x02\x73\x33\x09" +
	"\x61\x6d\x61\x7a\x6f\x6e\x61\x77\x73\x03\x63\x6f\x6d\x00\x00\x01" +
	"\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\x11\x9e\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x4c\x66\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\xda\x10\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\x01\x3e\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\x88\x68\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\x66\x9e\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\x5f\x28\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x8e\x4e\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x36\xe7" +
	"\x84\xf0\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x34\xd8" +
	"\x92\x45\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x8f\xa6\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x36\xe7" +
	"\xc0\xd0\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\xfe\x28\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\xaa\x3d\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x4e\x56\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd9" +
	"\xea\xb0\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x6d\xed\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x04\x00\x04\x34\xd8" +
	"\x28\x00\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x34\xd9" +
	"\xe9\x78\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x34\xd9" +
	"\x6e\x9e\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x34\xd9" +
	"\x45\x86\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x34\xd8" +
	"\x30\x38\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x36\xe7" +
	"\xc6\xa8\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x00\x00\x04\x03\x05" +
	"\x01\x9d\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd9" +
	"\xa8\xe8\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd9" +
	"\x64\xa6\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd8" +
	"\x3c\x48\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd8" +
	"\x35\x20\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd9" +
	"\x54\xf6\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd9" +
	"\x5d\x36\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x34\xd9" +
	"\x30\x36\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x05\x00\x04\x36\xe7" +
	"\x83\x90"

// Regression test for https://github.com/moby/moby/issues/44575
func TestOversizedDNSReply(t *testing.T) {
	srv, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer srv.Close()
	go func() {
		buf := make([]byte, 65536)
		for {
			n, src, err := srv.ReadFrom(buf)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			t.Logf("[<-%v]\n%s", src, hex.Dump(buf[:n]))
			if n < 2 {
				continue
			}
			resp := []byte(oversizedDNSReplyMsg)
			resp[0], resp[1] = buf[0], buf[1] // Copy query ID into response.
			_, err = srv.WriteTo(resp, src)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if err != nil {
				t.Log(err)
			}
		}
	}()

	srvAddr := srv.LocalAddr().(*net.UDPAddr)
	rsv := NewResolver("", true, noopDNSBackend{})
	rsv.SetExtServers([]extDNSEntry{
		{IPStr: srvAddr.IP.String(), port: uint16(srvAddr.Port), HostLoopback: true},
	})

	// The resolver logs lots of valuable info at level debug. Redirect it
	// to t.Log() so the log spew is emitted only if the test fails.
	defer redirectLogrusTo(t)()

	w := &tstwriter{network: srvAddr.Network()}
	q := new(dns.Msg).SetQuestion("s3.amazonaws.com.", dns.TypeA)
	rsv.serveDNS(w, q)
	resp := w.GetResponse()
	checkNonNullResponse(t, resp)
	t.Log("Response: ", resp.String())
	checkDNSResponseCode(t, resp, dns.RcodeSuccess)
	assert.Assert(t, len(resp.Answer) >= 1)
	checkDNSRRType(t, resp.Answer[0].Header().Rrtype, dns.TypeA)
}

func redirectLogrusTo(t *testing.T) func() {
	oldLevel, oldOut := logrus.StandardLogger().Level, logrus.StandardLogger().Out
	logrus.StandardLogger().SetLevel(logrus.DebugLevel)
	logrus.SetOutput(tlogWriter{t})
	return func() {
		logrus.StandardLogger().SetLevel(oldLevel)
		logrus.StandardLogger().SetOutput(oldOut)
	}
}

type tlogWriter struct{ t *testing.T }

func (w tlogWriter) Write(p []byte) (n int, err error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

type noopDNSBackend struct{ DNSBackend }

func (noopDNSBackend) ResolveName(name string, iplen int) ([]net.IP, bool) { return nil, false }

func (noopDNSBackend) ExecFunc(f func()) error { f(); return nil }

func (noopDNSBackend) NdotsSet() bool { return false }

func (noopDNSBackend) HandleQueryResp(name string, ip net.IP) {}

func TestReplySERVFAIL(t *testing.T) {
	cases := []struct {
		name     string
		q        *dns.Msg
		proxyDNS bool
	}{
		{
			name: "InternalError",
			q:    new(dns.Msg).SetQuestion("_sip._tcp.example.com.", dns.TypeSRV),
		},
		{
			name: "ProxyDNS=false",
			q:    new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
		},
		{
			name:     "ProxyDNS=true", // No extDNS servers configured -> no answer from any upstream
			q:        new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			defer redirectLogrusTo(t)

			rsv := NewResolver("", tt.proxyDNS, badSRVDNSBackend{})
			w := &tstwriter{}
			rsv.serveDNS(w, tt.q)
			resp := w.GetResponse()
			checkNonNullResponse(t, resp)
			t.Log("Response: ", resp.String())
			checkDNSResponseCode(t, resp, dns.RcodeServerFailure)
		})
	}
}

type badSRVDNSBackend struct{ noopDNSBackend }

func (badSRVDNSBackend) ResolveService(name string) ([]*net.SRV, []net.IP) {
	return []*net.SRV{nil, nil, nil}, nil // Mismatched slice lengths
}

func TestProxyNXDOMAIN(t *testing.T) {
	mockSOA, err := dns.NewRR(".	86367	IN	SOA	a.root-servers.net. nstld.verisign-grs.com. 2023051800 1800 900 604800 86400\n")
	assert.NilError(t, err)
	assert.Assert(t, mockSOA != nil)

	serveStarted := make(chan struct{})
	srv := &dns.Server{
		Net:  "udp",
		Addr: "127.0.0.1:0",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg).SetRcode(r, dns.RcodeNameError)
			msg.Ns = append(msg.Ns, dns.Copy(mockSOA))
			w.WriteMsg(msg)
		}),
		NotifyStartedFunc: func() { close(serveStarted) },
	}
	serveDone := make(chan error, 1)
	go func() {
		defer close(serveDone)
		serveDone <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveDone:
		t.Fatal(err)
	case <-serveStarted:
	}

	defer func() {
		if err := srv.Shutdown(); err != nil {
			t.Error(err)
		}
		<-serveDone
	}()

	srvAddr := srv.PacketConn.LocalAddr().(*net.UDPAddr)
	rsv := NewResolver("", true, noopDNSBackend{})
	rsv.SetExtServers([]extDNSEntry{
		{IPStr: srvAddr.IP.String(), port: uint16(srvAddr.Port), HostLoopback: true},
	})

	// The resolver logs lots of valuable info at level debug. Redirect it
	// to t.Log() so the log spew is emitted only if the test fails.
	defer redirectLogrusTo(t)()

	w := &tstwriter{network: srvAddr.Network()}
	q := new(dns.Msg).SetQuestion("example.net.", dns.TypeA)
	rsv.serveDNS(w, q)
	resp := w.GetResponse()
	checkNonNullResponse(t, resp)
	t.Log("Response:\n" + resp.String())
	checkDNSResponseCode(t, resp, dns.RcodeNameError)
	assert.Assert(t, is.Len(resp.Answer, 0))
	assert.Assert(t, is.Len(resp.Ns, 1))
	assert.Equal(t, resp.Ns[0].String(), mockSOA.String())
}
