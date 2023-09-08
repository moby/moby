package libnetwork

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/types"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// DNSBackend represents a backend DNS resolver used for DNS name
// resolution. All the queries to the resolver are forwarded to the
// backend resolver.
type DNSBackend interface {
	// ResolveName resolves a service name to an IPv4 or IPv6 address by searching
	// the networks the sandbox is connected to. For IPv6 queries, second return
	// value will be true if the name exists in docker domain but doesn't have an
	// IPv6 address. Such queries shouldn't be forwarded to external nameservers.
	ResolveName(name string, iplen int) ([]net.IP, bool)
	// ResolveIP returns the service name for the passed in IP. IP is in reverse dotted
	// notation; the format used for DNS PTR records
	ResolveIP(name string) string
	// ResolveService returns all the backend details about the containers or hosts
	// backing a service. Its purpose is to satisfy an SRV query
	ResolveService(name string) ([]*net.SRV, []net.IP)
	// ExecFunc allows a function to be executed in the context of the backend
	// on behalf of the resolver.
	ExecFunc(f func()) error
	// NdotsSet queries the backends ndots dns option settings
	NdotsSet() bool
	// HandleQueryResp passes the name & IP from a response to the backend. backend
	// can use it to maintain any required state about the resolution
	HandleQueryResp(name string, ip net.IP)
}

const (
	dnsPort       = "53"
	ptrIPv4domain = ".in-addr.arpa."
	ptrIPv6domain = ".ip6.arpa."
	respTTL       = 600
	maxExtDNS     = 3 // max number of external servers to try
	extIOTimeout  = 4 * time.Second
	maxConcurrent = 1024
	logInterval   = 2 * time.Second
)

type extDNSEntry struct {
	IPStr        string
	port         uint16 // for testing
	HostLoopback bool
}

// Resolver is the embedded DNS server in Docker. It operates by listening on
// the container's loopback interface for DNS queries.
type Resolver struct {
	backend       DNSBackend
	extDNSList    [maxExtDNS]extDNSEntry
	server        *dns.Server
	conn          *net.UDPConn
	tcpServer     *dns.Server
	tcpListen     *net.TCPListener
	err           error
	listenAddress string
	proxyDNS      bool
	startCh       chan struct{}
	logger        *logrus.Logger

	fwdSem      *semaphore.Weighted // Limit the number of concurrent external DNS requests in-flight
	logInverval rate.Sometimes      // Rate-limit logging about hitting the fwdSem limit
}

// NewResolver creates a new instance of the Resolver
func NewResolver(address string, proxyDNS bool, backend DNSBackend) *Resolver {
	return &Resolver{
		backend:       backend,
		proxyDNS:      proxyDNS,
		listenAddress: address,
		err:           fmt.Errorf("setup not done yet"),
		startCh:       make(chan struct{}, 1),
		fwdSem:        semaphore.NewWeighted(maxConcurrent),
		logInverval:   rate.Sometimes{Interval: logInterval},
	}
}

func (r *Resolver) log() *logrus.Logger {
	if r.logger == nil {
		return log.G(context.TODO()).Logger
	}
	return r.logger
}

// SetupFunc returns the setup function that should be run in the container's
// network namespace.
func (r *Resolver) SetupFunc(port int) func() {
	return func() {
		var err error

		// DNS operates primarily on UDP
		r.conn, err = net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(r.listenAddress),
			Port: port,
		})
		if err != nil {
			r.err = fmt.Errorf("error in opening name server socket %v", err)
			return
		}

		// Listen on a TCP as well
		r.tcpListen, err = net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.ParseIP(r.listenAddress),
			Port: port,
		})
		if err != nil {
			r.err = fmt.Errorf("error in opening name TCP server socket %v", err)
			return
		}
		r.err = nil
	}
}

// Start starts the name server for the container.
func (r *Resolver) Start() error {
	r.startCh <- struct{}{}
	defer func() { <-r.startCh }()

	// make sure the resolver has been setup before starting
	if r.err != nil {
		return r.err
	}

	if err := r.setupIPTable(); err != nil {
		return fmt.Errorf("setting up IP table rules failed: %v", err)
	}

	s := &dns.Server{Handler: dns.HandlerFunc(r.serveDNS), PacketConn: r.conn}
	r.server = s
	go func() {
		if err := s.ActivateAndServe(); err != nil {
			r.log().WithError(err).Error("[resolver] failed to start PacketConn DNS server")
		}
	}()

	tcpServer := &dns.Server{Handler: dns.HandlerFunc(r.serveDNS), Listener: r.tcpListen}
	r.tcpServer = tcpServer
	go func() {
		if err := tcpServer.ActivateAndServe(); err != nil {
			r.log().WithError(err).Error("[resolver] failed to start TCP DNS server")
		}
	}()
	return nil
}

// Stop stops the name server for the container. A stopped resolver can be
// reused after running the SetupFunc again.
func (r *Resolver) Stop() {
	r.startCh <- struct{}{}
	defer func() { <-r.startCh }()

	if r.server != nil {
		r.server.Shutdown() //nolint:errcheck
	}
	if r.tcpServer != nil {
		r.tcpServer.Shutdown() //nolint:errcheck
	}
	r.conn = nil
	r.tcpServer = nil
	r.err = fmt.Errorf("setup not done yet")
	r.fwdSem = semaphore.NewWeighted(maxConcurrent)
}

// SetExtServers configures the external nameservers the resolver should use
// when forwarding queries.
func (r *Resolver) SetExtServers(extDNS []extDNSEntry) {
	l := len(extDNS)
	if l > maxExtDNS {
		l = maxExtDNS
	}
	for i := 0; i < l; i++ {
		r.extDNSList[i] = extDNS[i]
	}
}

// NameServer returns the IP of the DNS resolver for the containers.
func (r *Resolver) NameServer() string {
	return r.listenAddress
}

// ResolverOptions returns resolv.conf options that should be set.
func (r *Resolver) ResolverOptions() []string {
	return []string{"ndots:0"}
}

//nolint:gosec // The RNG is not used in a security-sensitive context.
var (
	shuffleRNG   = rand.New(rand.NewSource(time.Now().Unix()))
	shuffleRNGMu sync.Mutex
)

func shuffleAddr(addr []net.IP) []net.IP {
	shuffleRNGMu.Lock()
	defer shuffleRNGMu.Unlock()
	for i := len(addr) - 1; i > 0; i-- {
		r := shuffleRNG.Intn(i + 1) //nolint:gosec // gosec complains about the use of rand here. It should be fine.
		addr[i], addr[r] = addr[r], addr[i]
	}
	return addr
}

func createRespMsg(query *dns.Msg) *dns.Msg {
	resp := &dns.Msg{}
	resp.SetReply(query)
	resp.RecursionAvailable = true

	return resp
}

func (r *Resolver) handleMXQuery(query *dns.Msg) (*dns.Msg, error) {
	name := query.Question[0].Name
	addrv4, _ := r.backend.ResolveName(name, types.IPv4)
	addrv6, _ := r.backend.ResolveName(name, types.IPv6)

	if addrv4 == nil && addrv6 == nil {
		return nil, nil
	}

	// We were able to resolve the name. Respond with an empty list with
	// RcodeSuccess/NOERROR so that email clients can treat it as "implicit MX"
	// [RFC 5321 Section-5.1] and issue a Type A/AAAA query for the name.

	resp := createRespMsg(query)
	return resp, nil
}

func (r *Resolver) handleIPQuery(query *dns.Msg, ipType int) (*dns.Msg, error) {
	var (
		addr     []net.IP
		ipv6Miss bool
		name     = query.Question[0].Name
	)
	addr, ipv6Miss = r.backend.ResolveName(name, ipType)

	if addr == nil && ipv6Miss {
		// Send a reply without any Answer sections
		r.log().Debugf("[resolver] lookup name %s present without IPv6 address", name)
		resp := createRespMsg(query)
		return resp, nil
	}
	if addr == nil {
		return nil, nil
	}

	r.log().Debugf("[resolver] lookup for %s: IP %v", name, addr)

	resp := createRespMsg(query)
	if len(addr) > 1 {
		addr = shuffleAddr(addr)
	}
	if ipType == types.IPv4 {
		for _, ip := range addr {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: respTTL},
				A:   ip,
			})
		}
	} else {
		for _, ip := range addr {
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: respTTL},
				AAAA: ip,
			})
		}
	}
	return resp, nil
}

func (r *Resolver) handlePTRQuery(query *dns.Msg) (*dns.Msg, error) {
	ptr := query.Question[0].Name
	name, after, found := strings.Cut(ptr, ptrIPv4domain)
	if !found || after != "" {
		name, after, found = strings.Cut(ptr, ptrIPv6domain)
	}
	if !found || after != "" {
		// Not a known IPv4 or IPv6 PTR domain.
		// Maybe the external DNS servers know what to do with the query?
		return nil, nil
	}

	host := r.backend.ResolveIP(name)
	if host == "" {
		return nil, nil
	}

	r.log().Debugf("[resolver] lookup for IP %s: name %s", name, host)
	fqdn := dns.Fqdn(host)

	resp := createRespMsg(query)
	resp.Answer = append(resp.Answer, &dns.PTR{
		Hdr: dns.RR_Header{Name: ptr, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: respTTL},
		Ptr: fqdn,
	})
	return resp, nil
}

func (r *Resolver) handleSRVQuery(query *dns.Msg) (*dns.Msg, error) {
	svc := query.Question[0].Name
	srv, ip := r.backend.ResolveService(svc)

	if len(srv) == 0 {
		return nil, nil
	}
	if len(srv) != len(ip) {
		return nil, fmt.Errorf("invalid reply for SRV query %s", svc)
	}

	resp := createRespMsg(query)

	for i, r := range srv {
		resp.Answer = append(resp.Answer, &dns.SRV{
			Hdr:    dns.RR_Header{Name: svc, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: respTTL},
			Port:   r.Port,
			Target: r.Target,
		})
		resp.Extra = append(resp.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: r.Target, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: respTTL},
			A:   ip[i],
		})
	}
	return resp, nil
}

func (r *Resolver) serveDNS(w dns.ResponseWriter, query *dns.Msg) {
	var (
		resp *dns.Msg
		err  error
	)

	if query == nil || len(query.Question) == 0 {
		return
	}

	queryName := query.Question[0].Name
	queryType := query.Question[0].Qtype

	switch queryType {
	case dns.TypeA:
		resp, err = r.handleIPQuery(query, types.IPv4)
	case dns.TypeAAAA:
		resp, err = r.handleIPQuery(query, types.IPv6)
	case dns.TypeMX:
		resp, err = r.handleMXQuery(query)
	case dns.TypePTR:
		resp, err = r.handlePTRQuery(query)
	case dns.TypeSRV:
		resp, err = r.handleSRVQuery(query)
	default:
		r.log().Debugf("[resolver] query type %s is not supported by the embedded DNS and will be forwarded to external DNS", dns.TypeToString[queryType])
	}

	reply := func(msg *dns.Msg) {
		if err = w.WriteMsg(msg); err != nil {
			r.log().WithError(err).Errorf("[resolver] failed to write response")
		}
	}

	if err != nil {
		r.log().WithError(err).Errorf("[resolver] failed to handle query: %s (%s)", queryName, dns.TypeToString[queryType])
		reply(new(dns.Msg).SetRcode(query, dns.RcodeServerFailure))
		return
	}

	if resp != nil {
		// We are the authoritative DNS server for this request so it's
		// on us to truncate the response message to the size limit
		// negotiated by the client.
		maxSize := dns.MinMsgSize
		if w.LocalAddr().Network() == "tcp" {
			maxSize = dns.MaxMsgSize
		} else {
			if optRR := query.IsEdns0(); optRR != nil {
				if udpsize := int(optRR.UDPSize()); udpsize > maxSize {
					maxSize = udpsize
				}
			}
		}
		resp.Truncate(maxSize)
		reply(resp)
		return
	}

	if r.proxyDNS {
		// If the user sets ndots > 0 explicitly and the query is
		// in the root domain don't forward it out. We will return
		// failure and let the client retry with the search domain
		// attached.
		if (queryType == dns.TypeA || queryType == dns.TypeAAAA) && r.backend.NdotsSet() &&
			!strings.Contains(strings.TrimSuffix(queryName, "."), ".") {
			resp = createRespMsg(query)
		} else {
			resp = r.forwardExtDNS(w.LocalAddr().Network(), query)
		}
	}

	if resp == nil {
		// We were unable to get an answer from any of the upstream DNS
		// servers or the backend doesn't support proxying DNS requests.
		resp = new(dns.Msg).SetRcode(query, dns.RcodeServerFailure)
	}
	reply(resp)
}

const defaultPort = "53"

func (r *Resolver) dialExtDNS(proto string, server extDNSEntry) (net.Conn, error) {
	port := defaultPort
	if server.port != 0 {
		port = strconv.FormatUint(uint64(server.port), 10)
	}
	addr := net.JoinHostPort(server.IPStr, port)

	if server.HostLoopback {
		return net.DialTimeout(proto, addr, extIOTimeout)
	}

	var (
		extConn net.Conn
		dialErr error
	)
	err := r.backend.ExecFunc(func() {
		extConn, dialErr = net.DialTimeout(proto, addr, extIOTimeout)
	})
	if err != nil {
		return nil, err
	}
	if dialErr != nil {
		return nil, dialErr
	}

	return extConn, nil
}

func (r *Resolver) forwardExtDNS(proto string, query *dns.Msg) *dns.Msg {
	for _, extDNS := range r.extDNSList {
		if extDNS.IPStr == "" {
			break
		}

		// limits the number of outstanding concurrent queries.
		ctx, cancel := context.WithTimeout(context.Background(), extIOTimeout)
		err := r.fwdSem.Acquire(ctx, 1)
		cancel()
		if err != nil {
			r.logInverval.Do(func() {
				r.log().Errorf("[resolver] more than %v concurrent queries", maxConcurrent)
			})
			return new(dns.Msg).SetRcode(query, dns.RcodeRefused)
		}
		resp := func() *dns.Msg {
			defer r.fwdSem.Release(1)
			return r.exchange(proto, extDNS, query)
		}()
		if resp == nil {
			continue
		}

		switch resp.Rcode {
		case dns.RcodeServerFailure, dns.RcodeRefused:
			// Server returned FAILURE: continue with the next external DNS server
			// Server returned REFUSED: this can be a transitional status, so continue with the next external DNS server
			r.log().Debugf("[resolver] external DNS %s:%s returned failure:\n%s", proto, extDNS.IPStr, resp)
			continue
		}
		answers := 0
		for _, rr := range resp.Answer {
			h := rr.Header()
			switch h.Rrtype {
			case dns.TypeA:
				answers++
				ip := rr.(*dns.A).A
				r.log().Debugf("[resolver] received A record %q for %q from %s:%s", ip, h.Name, proto, extDNS.IPStr)
				r.backend.HandleQueryResp(h.Name, ip)
			case dns.TypeAAAA:
				answers++
				ip := rr.(*dns.AAAA).AAAA
				r.log().Debugf("[resolver] received AAAA record %q for %q from %s:%s", ip, h.Name, proto, extDNS.IPStr)
				r.backend.HandleQueryResp(h.Name, ip)
			}
		}
		if len(resp.Answer) == 0 {
			r.log().Debugf("[resolver] external DNS %s:%s returned response with no answers:\n%s", proto, extDNS.IPStr, resp)
		}
		resp.Compress = true
		return resp
	}

	return nil
}

func (r *Resolver) exchange(proto string, extDNS extDNSEntry, query *dns.Msg) *dns.Msg {
	extConn, err := r.dialExtDNS(proto, extDNS)
	if err != nil {
		r.log().WithError(err).Warn("[resolver] connect failed")
		return nil
	}
	defer extConn.Close()

	logger := r.log().WithFields(log.Fields{
		"dns-server":  extConn.RemoteAddr().Network() + ":" + extConn.RemoteAddr().String(),
		"client-addr": extConn.LocalAddr().Network() + ":" + extConn.LocalAddr().String(),
		"question":    query.Question[0].String(),
	})
	logger.Debug("[resolver] forwarding query")

	resp, _, err := (&dns.Client{
		Timeout: extIOTimeout,
		// Following the robustness principle, make a best-effort
		// attempt to receive oversized response messages without
		// truncating them on our end to forward verbatim to the client.
		// Some DNS servers (e.g. Mikrotik RouterOS) don't support
		// EDNS(0) and may send replies over UDP longer than 512 bytes
		// regardless of what size limit, if any, was advertized in the
		// query message. Note that ExchangeWithConn will override this
		// value if it detects an EDNS OPT record in query so only
		// oversized replies to non-EDNS queries will benefit.
		UDPSize: dns.MaxMsgSize,
	}).ExchangeWithConn(query, &dns.Conn{Conn: extConn})
	if err != nil {
		r.log().WithError(err).Errorf("[resolver] failed to query DNS server: %s, query: %s", extConn.RemoteAddr().String(), query.Question[0].String())
		return nil
	}

	if resp == nil {
		// Should be impossible, so make noise if it happens anyway.
		logger.Error("[resolver] external DNS returned empty response")
	}
	return resp
}
