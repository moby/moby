//go:build linux

package proxyprovider

import (
	"bufio"
	"container/list"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"hash"
	"io"
	"maps"
	"math/big"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/pkg/oci"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/buildkit/util/network/netpool"
	"github.com/moby/buildkit/util/urlutil"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	proxyCACertLifetime        = 10 * 365 * 24 * time.Hour
	proxyLeafCertLifetime      = 24 * time.Hour
	proxyLeafCertRefreshBefore = time.Hour
	proxyCertCacheMaxEntries   = 1024
)

type Opt struct {
	Root                 string
	PoolSize             int
	EgressProviders      map[pb.NetMode]network.Provider
	OwnedEgressProviders []network.Provider
}

func Supported() bool {
	return true
}

func New(opt Opt) (network.ProxyProvider, error) {
	cleanOldNamespaces(opt.Root)
	certPEM, ca, key, err := newCA()
	if err != nil {
		return nil, err
	}
	p := &provider{
		root:                 opt.Root,
		caPEM:                certPEM,
		ca:                   ca,
		caKey:                key,
		certs:                map[string]*certCacheEntry{},
		lru:                  list.New(),
		egressProviders:      maps.Clone(opt.EgressProviders),
		ownedEgressProviders: slices.Clone(opt.OwnedEgressProviders),
		transport:            newProxyTransport(),
	}
	p.pool = netpool.New(netpool.Opt[*proxyNS]{
		Name:       "proxy network namespace",
		TargetSize: opt.PoolSize,
		New:        p.newNS,
		Release: func(ns *proxyNS) error {
			return ns.release()
		},
	})
	go p.pool.Fill(context.TODO())
	return p, nil
}

func newProxyTransport() *http.Transport {
	return &http.Transport{
		Proxy:              nil,
		DisableCompression: true,
		ForceAttemptHTTP2:  true,
	}
}

type provider struct {
	root string
	next atomic.Uint32
	pool *netpool.Pool[*proxyNS]

	caPEM []byte
	ca    *x509.Certificate
	caKey *rsa.PrivateKey

	certsMu sync.Mutex
	certs   map[string]*certCacheEntry
	lru     *list.List

	egressProviders      map[pb.NetMode]network.Provider
	ownedEgressProviders []network.Provider
	transport            *http.Transport
}

type certCacheEntry struct {
	host    string
	cert    *tls.Certificate
	expires time.Time
	elem    *list.Element
}

func (p *provider) Close() error {
	err := p.pool.Close()
	p.transport.CloseIdleConnections()
	for _, provider := range p.ownedEgressProviders {
		if e := provider.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (p *provider) NewProxy(ctx context.Context, proxy *network.ProxyConfig) (_ network.ProxyNamespace, retErr error) {
	ns, err := p.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			_ = p.pool.Discard(ns)
		}
	}()
	if err := ns.startProxy(ctx, proxy); err != nil {
		return nil, err
	}
	return ns, nil
}

func (p *provider) newNS(ctx context.Context) (_ *proxyNS, retErr error) {
	n := p.next.Add(1)
	id := identity.NewID()
	nsPath, err := createNetNS(p.root, id+"-exec")
	if err != nil {
		return nil, err
	}
	proxyNSPath, err := createNetNS(p.root, id+"-proxy")
	if err != nil {
		_ = unmountNetNS(nsPath)
		_ = deleteNetNS(nsPath)
		return nil, err
	}
	ns := &proxyNS{
		provider:    p,
		nsPath:      nsPath,
		proxyNSPath: proxyNSPath,
		hostName:    ifName("bkpxh", id),
		ctrName:     ifName("bkpxc", id),
		hostIP:      proxyHostIP(n),
		ctrIP:       proxyContainerIP(n),
		prefix:      proxyPrefix(),
	}
	defer func() {
		if retErr != nil {
			_ = ns.Close()
		}
	}()
	if err := ns.setupVeth(); err != nil {
		return nil, err
	}
	return ns, nil
}

type proxyNS struct {
	provider    *provider
	nsPath      string
	proxyNSPath string
	hostName    string
	ctrName     string
	hostIP      net.IP
	ctrIP       net.IP
	prefix      int

	server    *http.Server
	ln        net.Listener
	egressNS  network.Namespace
	transport *http.Transport
}

func (n *proxyNS) Set(s *specs.Spec) error {
	return oci.WithLinuxNamespace(specs.LinuxNamespace{
		Type: specs.NetworkNamespace,
		Path: n.nsPath,
	})(nil, nil, nil, s)
}

func (n *proxyNS) Close() error {
	if err := n.stopProxy(); err != nil {
		if n.provider != nil && n.provider.pool != nil {
			_ = n.provider.pool.Discard(n)
		}
		return err
	}
	if n.provider != nil && n.provider.pool != nil {
		n.provider.pool.Put(n)
		return nil
	}
	return n.release()
}

func (n *proxyNS) stopProxy() error {
	var err error
	if n.server != nil {
		if e := n.server.Close(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			err = errors.WithStack(e)
		}
	}
	n.server = nil
	n.ln = nil
	if n.transport != nil {
		n.transport.CloseIdleConnections()
		n.transport = nil
	}
	if n.egressNS != nil {
		if e := n.egressNS.Close(); e != nil && err == nil {
			err = e
		}
		n.egressNS = nil
	}
	return err
}

func (n *proxyNS) release() error {
	var err error
	if e := n.stopProxy(); e != nil {
		err = e
	}
	if e := n.deleteVeth(); e != nil && err == nil {
		err = e
	}
	if e := unmountNetNS(n.nsPath); e != nil && err == nil {
		err = e
	}
	if e := deleteNetNS(n.nsPath); e != nil && err == nil {
		err = e
	}
	if e := unmountNetNS(n.proxyNSPath); e != nil && err == nil {
		err = e
	}
	if e := deleteNetNS(n.proxyNSPath); e != nil && err == nil {
		err = e
	}
	return err
}

func (n *proxyNS) Sample() (*resourcestypes.NetworkSample, error) {
	return nil, nil
}

func (n *proxyNS) ProxyEnv() []string {
	proxy := "http://" + n.ln.Addr().String()
	noProxy := "127.0.0.1,localhost,::1"
	return []string{
		"HTTP_PROXY=" + proxy,
		"HTTPS_PROXY=" + proxy,
		"http_proxy=" + proxy,
		"https_proxy=" + proxy,
		"NO_PROXY=" + noProxy,
		"no_proxy=" + noProxy,
	}
}

func (n *proxyNS) ProxyCACert() []byte {
	return n.provider.caPEM
}

func (n *proxyNS) setupVeth() error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: n.hostName},
		PeerName:  n.ctrName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return errors.WithStack(err)
	}
	host, err := netlink.LinkByName(n.hostName)
	if err != nil {
		return errors.WithStack(err)
	}
	peer, err := netlink.LinkByName(n.ctrName)
	if err != nil {
		return errors.WithStack(err)
	}
	target, err := netns.GetFromPath(n.nsPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer target.Close()
	proxyTarget, err := netns.GetFromPath(n.proxyNSPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer proxyTarget.Close()
	if err := netlink.LinkSetNsFd(host, int(proxyTarget)); err != nil {
		return errors.WithStack(err)
	}
	if err := netlink.LinkSetNsFd(peer, int(target)); err != nil {
		return errors.WithStack(err)
	}
	ph, err := netlink.NewHandleAt(proxyTarget)
	if err != nil {
		return errors.WithStack(err)
	}
	defer ph.Close()
	host, err = ph.LinkByName(n.hostName)
	if err != nil {
		return errors.WithStack(err)
	}
	hostAddr := &netlink.Addr{IPNet: &net.IPNet{IP: n.hostIP, Mask: net.CIDRMask(n.prefix, 32)}}
	if err := ph.AddrAdd(host, hostAddr); err != nil {
		return errors.WithStack(err)
	}
	if err := ph.LinkSetUp(host); err != nil {
		return errors.WithStack(err)
	}
	proxyLO, err := ph.LinkByName("lo")
	if err != nil {
		return errors.WithStack(err)
	}
	if err := ph.LinkSetUp(proxyLO); err != nil {
		return errors.WithStack(err)
	}
	h, err := netlink.NewHandleAt(target)
	if err != nil {
		return errors.WithStack(err)
	}
	defer h.Close()
	peer, err = h.LinkByName(n.ctrName)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := h.LinkSetName(peer, "eth0"); err != nil {
		return errors.WithStack(err)
	}
	peer, err = h.LinkByName("eth0")
	if err != nil {
		return errors.WithStack(err)
	}
	ctrAddr := &netlink.Addr{IPNet: &net.IPNet{IP: n.ctrIP, Mask: net.CIDRMask(n.prefix, 32)}}
	if err := h.AddrAdd(peer, ctrAddr); err != nil {
		return errors.WithStack(err)
	}
	if err := h.LinkSetUp(peer); err != nil {
		return errors.WithStack(err)
	}
	lo, err := h.LinkByName("lo")
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(h.LinkSetUp(lo))
}

func (n *proxyNS) startProxy(ctx context.Context, proxy *network.ProxyConfig) error {
	if proxy == nil {
		return errors.New("proxy network config is required")
	}
	ln, err := listenInNetNS(ctx, n.proxyNSPath, "tcp4", net.JoinHostPort(n.hostIP.String(), "0"))
	if err != nil {
		return errors.WithStack(err)
	}
	n.ln = ln
	egressNS, err := n.egressNamespace(ctx, proxy.EgressMode)
	if err != nil {
		_ = ln.Close()
		n.ln = nil
		return err
	}
	dialer, ok := egressNS.(network.Dialer)
	if !ok {
		_ = egressNS.Close()
		_ = ln.Close()
		n.ln = nil
		return errors.Errorf("proxy egress network mode %s does not support dialing", proxy.EgressMode)
	}
	n.egressNS = egressNS
	transport := n.provider.transport.Clone()
	transport.DialContext = dialer.DialContext
	n.transport = transport
	handler := &proxyHandler{
		provider:  n.provider,
		policy:    proxy.Policy,
		capture:   proxy.Capture,
		transport: transport,
	}
	n.server = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		_ = n.server.Serve(ln)
	}()
	return nil
}

func (n *proxyNS) egressNamespace(ctx context.Context, mode pb.NetMode) (network.Namespace, error) {
	provider, ok := n.provider.egressProviders[mode]
	if !ok {
		return nil, errors.Errorf("unknown proxy egress network mode %s", mode)
	}
	ns, err := provider.New(ctx, "", network.NamespaceOptions{})
	if err != nil {
		return nil, err
	}
	return ns, nil
}

func listenInNetNS(ctx context.Context, nsPath, networkName, address string) (net.Listener, error) {
	var ln net.Listener
	if err := withNetNS(nsPath, func() error {
		l, err := (&net.ListenConfig{}).Listen(ctx, networkName, address)
		if err != nil {
			return errors.WithStack(err)
		}
		ln = l
		return nil
	}); err != nil {
		return nil, err
	}
	return ln, nil
}

func withNetNS(nsPath string, fn func() error) (retErr error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	orig, err := netns.Get()
	if err != nil {
		return errors.WithStack(err)
	}
	defer orig.Close()

	target, err := netns.GetFromPath(nsPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer target.Close()

	if err := netns.Set(target); err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		if err := netns.Set(orig); err != nil && retErr == nil {
			retErr = errors.WithStack(err)
		}
	}()

	return fn()
}

func (n *proxyNS) deleteVeth() error {
	target, err := netns.GetFromPath(n.proxyNSPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.WithStack(err)
	}
	defer target.Close()
	h, err := netlink.NewHandleAt(target)
	if err != nil {
		return errors.WithStack(err)
	}
	defer h.Close()
	link, err := h.LinkByName(n.hostName)
	if err != nil {
		var linkNotFound netlink.LinkNotFoundError
		if errors.As(err, &linkNotFound) {
			return nil
		}
		return errors.WithStack(err)
	}
	return errors.WithStack(h.LinkDel(link))
}

type proxyHandler struct {
	provider  *provider
	policy    network.ProxyPolicy
	capture   *network.ProxyCapture
	transport *http.Transport
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r)
		return
	}
	if !r.URL.IsAbs() {
		r.URL.Scheme = "http"
		r.URL.Host = r.Host
	}
	if target, err := h.check(r.Context(), r.Method, r.URL.String()); err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	} else if target != nil {
		r.URL = target
		r.Host = target.Host
	}
	resp, err := h.roundTrip(r)
	if err != nil {
		h.recordRequest(r, http.StatusBadGateway, "")
		h.recordIncomplete(r, "upstream_error")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	h.recordRequest(r, resp.StatusCode, finalURL(r, resp))
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	tracker := newProxyBodyTracker(resp.Body)
	_, copyErr := io.Copy(w, tracker)
	h.recordResponse(r, resp, tracker, copyErr)
}

func (h *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	host := stripPort(r.Host)
	cert, err := h.provider.certForHost(host)
	if err != nil {
		return
	}
	tlsConn := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{*cert},
		NextProtos:   []string{"http/1.1"},
	})
	defer tlsConn.Close()
	if err := tlsConn.HandshakeContext(r.Context()); err != nil {
		return
	}
	br := bufio.NewReader(tlsConn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		req.URL.Scheme = "https"
		req.URL.Host = r.Host
		req.RequestURI = ""
		if target, err := h.check(req.Context(), req.Method, req.URL.String()); err != nil {
			_ = req.Body.Close()
			_, _ = io.WriteString(tlsConn, "HTTP/1.1 403 Forbidden\r\nContent-Length: 10\r\nConnection: close\r\n\r\nForbidden\n")
			return
		} else if target != nil {
			req.URL = target
			req.Host = target.Host
		}
		resp, err := h.roundTrip(req)
		if err != nil {
			_ = req.Body.Close()
			h.recordRequest(req, http.StatusBadGateway, "")
			h.recordIncomplete(req, "upstream_error")
			_, _ = fmt.Fprintf(tlsConn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(err.Error())+1, err.Error()+"\n")
			return
		}
		h.recordRequest(req, resp.StatusCode, finalURL(req, resp))
		prepareMITMResponse(req, resp)
		tracker := newProxyBodyTracker(resp.Body)
		resp.Body = tracker
		if err := resp.Write(tlsConn); err != nil {
			h.recordResponse(req, resp, tracker, err)
			resp.Body.Close()
			return
		}
		resp.Body.Close()
		h.recordResponse(req, resp, tracker, nil)
		if resp.Close || req.Close {
			return
		}
	}
}

func prepareMITMResponse(req *http.Request, resp *http.Response) {
	// Response.Write uses resp.Proto for the status line. In the MITM
	// path, resp describes the upstream fetch, so align it to the
	// client-facing request we intercepted.
	resp.Proto = req.Proto
	resp.ProtoMajor = req.ProtoMajor
	resp.ProtoMinor = req.ProtoMinor
	resp.Close = resp.Close || req.Close || !req.ProtoAtLeast(1, 1)
	if resp.ContentLength < 0 && len(resp.TransferEncoding) == 0 {
		// Response.Write marks its internal response clone as close-delimited
		// for this case, but it does not update resp.Close. Mirror that here
		// so handleConnect closes the client-facing TLS connection after the
		// upstream body is copied.
		resp.Close = true
	}
}

type proxyBodyTracker struct {
	body    io.ReadCloser
	hash    hash.Hash
	readErr error
}

func newProxyBodyTracker(body io.ReadCloser) *proxyBodyTracker {
	return &proxyBodyTracker{
		body: body,
		hash: sha256.New(),
	}
}

func (t *proxyBodyTracker) Read(p []byte) (int, error) {
	n, err := t.body.Read(p)
	if n > 0 {
		_, _ = t.hash.Write(p[:n])
	}
	if err != nil && !errors.Is(err, io.EOF) && t.readErr == nil {
		t.readErr = err
	}
	return n, err
}

func (t *proxyBodyTracker) Close() error {
	return t.body.Close()
}

func (t *proxyBodyTracker) Digest() digest.Digest {
	return digest.NewDigestFromHex(string(digest.SHA256), hex.EncodeToString(t.hash.Sum(nil)))
}

func (h *proxyHandler) recordResponse(req *http.Request, resp *http.Response, tracker *proxyBodyTracker, copyErr error) {
	if h.capture == nil {
		return
	}
	reason := proxyIncompleteReason(req, resp, tracker, copyErr)
	if reason != "" {
		h.recordIncomplete(req, reason)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	h.capture.AddMaterial(network.ProxyMaterial{
		URL:    captureURL(req.URL.String()),
		Digest: tracker.Digest(),
	})
}

func (h *proxyHandler) recordRequest(req *http.Request, statusCode int, redirectTarget string) {
	if h.capture == nil {
		return
	}
	h.capture.AddRequest(network.ProxyRequest{
		Method:         req.Method,
		URL:            captureURL(req.URL.String()),
		RedirectTarget: captureURL(redirectTarget),
		StatusCode:     statusCode,
	})
}

func (h *proxyHandler) recordIncomplete(req *http.Request, reason string) {
	if h.capture == nil {
		return
	}
	h.capture.AddIncomplete(network.ProxyIncomplete{
		Method: req.Method,
		URL:    captureURL(req.URL.String()),
		Reason: reason,
	})
}

func proxyIncompleteReason(req *http.Request, resp *http.Response, tracker *proxyBodyTracker, copyErr error) string {
	if req.Method != http.MethodGet {
		return "method_not_materializable"
	}
	if req.Header.Get("Range") != "" || resp.StatusCode == http.StatusPartialContent {
		return "partial_response"
	}
	if resp.Uncompressed {
		return "response_transformed"
	}
	if copyErr != nil || tracker.readErr != nil {
		return "body_read_failed"
	}
	if resp.StatusCode >= 400 {
		return "unsuccessful_response"
	}
	return ""
}

func finalURL(req *http.Request, resp *http.Response) string {
	location := resp.Header.Get("Location")
	if location == "" {
		return ""
	}
	u, err := req.URL.Parse(location)
	if err != nil {
		return location
	}
	return u.String()
}

func redactURL(s string) string {
	if s == "" {
		return ""
	}
	return urlutil.RedactCredentials(s)
}

func captureURL(s string) string {
	if s == "" {
		return ""
	}
	u, err := neturl.Parse(s)
	if err == nil && u.IsAbs() {
		port := u.Port()
		if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
			host := u.Hostname()
			if strings.Contains(host, ":") {
				host = "[" + host + "]"
			}
			u.Host = host
		}
		return redactURL(u.String())
	}
	return redactURL(s)
}

func (h *proxyHandler) roundTrip(r *http.Request) (*http.Response, error) {
	stripProxyHeaders(r.Header)
	r.Header.Del("Accept-Encoding")
	r.RequestURI = ""
	return h.transport.RoundTrip(r.WithContext(context.WithoutCancel(r.Context())))
}

func (h *proxyHandler) check(ctx context.Context, method, rawURL string) (*neturl.URL, error) {
	if h.policy == nil {
		return nil, nil
	}
	redactedURL := redactURL(rawURL)
	op := &pb.Op{
		Op: &pb.Op_Source{
			Source: &pb.SourceOp{
				Identifier: redactedURL,
			},
		},
	}
	if _, err := h.policy.Evaluate(ctx, op); err != nil {
		return nil, err
	}
	source := op.GetSource()
	target := source.Identifier
	converted := target != redactedURL || len(source.Attrs) != 0
	if !converted {
		return nil, nil
	}
	if method != http.MethodGet {
		return nil, errors.Errorf("source policy converted proxy request %q, but conversion is only supported for GET", redactedURL)
	}
	if len(source.Attrs) != 0 {
		return nil, errors.Errorf("source policy converted proxy request %q with attrs, but proxy conversion only supports URL updates", redactedURL)
	}
	u, err := neturl.Parse(target)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing converted proxy request URL %q", redactURL(target))
	}
	if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.Errorf("source policy converted proxy request to unsupported URL %q", redactURL(target))
	}
	return u, nil
}

func newCA() ([]byte, *x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "BuildKit exec proxy"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(proxyCACertLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return pemBytes, cert, key, nil
}

func (p *provider) certForHost(host string) (*tls.Certificate, error) {
	p.certsMu.Lock()
	defer p.certsMu.Unlock()

	if p.certs == nil {
		p.certs = map[string]*certCacheEntry{}
	}
	if p.lru == nil {
		p.lru = list.New()
	}
	now := time.Now()
	if ent, ok := p.certs[host]; ok {
		if now.Before(ent.expires) {
			p.lru.MoveToFront(ent.elem)
			return ent.cert, nil
		}
		p.removeCertEntry(ent)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	serialBytes := sha256.Sum256([]byte(host + time.Now().String()))
	serial := new(big.Int).SetBytes(serialBytes[:])
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(proxyLeafCertLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, p.ca, &key.PublicKey, p.caKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ent := &certCacheEntry{
		host:    host,
		cert:    &cert,
		expires: tmpl.NotAfter.Add(-proxyLeafCertRefreshBefore),
	}
	ent.elem = p.lru.PushFront(ent)
	p.certs[host] = ent
	for len(p.certs) > proxyCertCacheMaxEntries {
		oldest := p.lru.Back()
		if oldest == nil {
			break
		}
		p.removeCertEntry(oldest.Value.(*certCacheEntry))
	}
	return &cert, nil
}

func (p *provider) removeCertEntry(ent *certCacheEntry) {
	delete(p.certs, ent.host)
	p.lru.Remove(ent.elem)
}

func cleanOldNamespaces(root string) {
	nsDir := filepath.Join(root, "net/proxy")
	dirEntries, err := os.ReadDir(nsDir)
	if err != nil {
		bklog.L.Debugf("could not read %q for cleanup: %s", nsDir, err)
		return
	}
	go func() {
		for _, d := range dirEntries {
			nsPath := filepath.Join(nsDir, d.Name())
			if err := unmountNetNS(nsPath); err != nil {
				bklog.L.Warningf("failed to unmount proxy network namespace %q left over from previous run: %s", d.Name(), err)
				continue
			}
			if err := deleteNetNS(nsPath); err != nil {
				bklog.L.Warningf("failed to remove proxy network namespace %q left over from previous run: %s", d.Name(), err)
			}
		}
	}()
}

func createNetNS(root, id string) (_ string, err error) {
	nsPath := filepath.Join(root, "net/proxy", id)
	if err := os.MkdirAll(filepath.Dir(nsPath), 0700); err != nil {
		return "", errors.WithStack(err)
	}
	f, err := os.Create(nsPath)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if err := f.Close(); err != nil {
		return "", errors.WithStack(err)
	}
	defer func() {
		if err != nil {
			_ = deleteNetNS(nsPath)
		}
	}()
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		runtimeLockOSThread()
		if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
			errCh <- errors.WithStack(err)
			return
		}
		if err := syscall.Mount(fmt.Sprintf("/proc/self/task/%d/ns/net", syscall.Gettid()), nsPath, "", syscall.MS_BIND, ""); err != nil {
			errCh <- errors.WithStack(err)
			return
		}
	}()
	if err := <-errCh; err != nil {
		return "", err
	}
	return nsPath, nil
}

func unmountNetNS(nsPath string) error {
	if err := unix.Unmount(nsPath, unix.MNT_DETACH); err != nil {
		if !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOENT) {
			return errors.Wrap(err, "error unmounting network namespace")
		}
	}
	return nil
}

func deleteNetNS(nsPath string) error {
	if err := os.Remove(nsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrapf(err, "error removing network namespace %s", nsPath)
	}
	return nil
}

func runtimeLockOSThread() {
	runtime.LockOSThread()
}

func proxyHostIP(n uint32) net.IP {
	return proxyIP(n, 1)
}

func proxyContainerIP(n uint32) net.IP {
	return proxyIP(n, 2)
}

func proxyPrefix() int {
	return 30
}

func proxyIP(n uint32, offset byte) net.IP {
	block := n % 16384
	return net.IPv4(10, 89, byte(block/64), byte((block%64)*4)+offset)
}

func ifName(prefix, id string) string {
	const ifNameMaxLen = 15
	maxIDLen := ifNameMaxLen - len(prefix)
	if len(id) > maxIDLen {
		id = id[:maxIDLen]
	}
	return prefix + id
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}
	return strings.Trim(hostport, "[]")
}

func stripProxyHeaders(h http.Header) {
	for _, k := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		h.Del(k)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
