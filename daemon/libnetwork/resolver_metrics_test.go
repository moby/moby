package libnetwork

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	gometrics "github.com/docker/go-metrics"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	queryMetric    = "libnetwork_resolver_query_duration_seconds"
	upstreamMetric = "libnetwork_resolver_upstream_request_duration_seconds"
	failoverMetric = "libnetwork_resolver_upstream_failovers_total"
	inFlightMetric = "libnetwork_resolver_upstream_in_flight_requests"
)

// newTestResolverMetrics returns a set of resolver metrics registered with a
// private registry, so that tests can check them without interference from
// other tests or from the daemon-wide metrics.
func newTestResolverMetrics(t *testing.T) (*resolverMetrics, prometheus.Gatherer) {
	t.Helper()
	ns := gometrics.NewNamespace("libnetwork", "resolver", nil)
	m := newResolverMetrics(ns)
	reg := prometheus.NewPedanticRegistry()
	assert.NilError(t, reg.Register(ns))
	return m, reg
}

// metricsSnapshot gathers all series from g into a map keyed by the series
// name in Prometheus text format, e.g. `name{label="value"}`. The value is
// the sample count for histograms, and the value for counters and gauges.
func metricsSnapshot(t *testing.T, g prometheus.Gatherer) map[string]float64 {
	t.Helper()
	families, err := g.Gather()
	assert.NilError(t, err)
	snapshot := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			var value float64
			switch {
			case m.GetHistogram() != nil:
				value = float64(m.GetHistogram().GetSampleCount())
			case m.GetCounter() != nil:
				value = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				value = m.GetGauge().GetValue()
			case m.GetSummary() != nil:
				value = float64(m.GetSummary().GetSampleCount())
			default:
				t.Fatalf("unexpected type for metric %s", f.GetName())
			}
			snapshot[seriesName(f.GetName(), m.GetLabel())] = value
		}
	}
	return snapshot
}

// seriesName formats a metric name and its label pairs the way they appear
// in the Prometheus text format.
func seriesName[L interface {
	GetName() string
	GetValue() string
}](name string, labels []L) string {
	if len(labels) == 0 {
		return name
	}
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('{')
	for i, l := range labels {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(l.GetName())
		sb.WriteString(`="`)
		sb.WriteString(l.GetValue())
		sb.WriteByte('"')
	}
	sb.WriteByte('}')
	return sb.String()
}

// ndotsDNSBackend is a backend for a container with a non-zero ndots option.
type ndotsDNSBackend struct{ noopDNSBackend }

func (ndotsDNSBackend) NdotsSet() bool { return true }

// failingWriter is a dns.ResponseWriter whose writes always fail.
type failingWriter struct{ tstwriter }

func (w *failingWriter) WriteMsg(*dns.Msg) error { return errors.New("write failed") }

func TestResolverMetrics(t *testing.T) {
	localBackend := newStaticDNSBackend()

	testcases := []struct {
		description string
		query       *dns.Msg
		proto       string     // Protocol the query arrives on; "udp" if empty.
		backend     DNSBackend // noopDNSBackend if nil.
		proxyDNS    bool
		upstreams   []dns.HandlerFunc // Fake upstream servers, in the order they're tried.
		// containerNS marks the upstream with the same index as reached from
		// the container's network namespace rather than the host's (so it's
		// used even when proxyDNS is false). Missing entries are host-loopback.
		containerNS []bool
		// timeout shortens the resolver's I/O timeout so that cases which
		// wait for it stay fast; extIOTimeout if zero.
		timeout time.Duration
		// limitReached leaves the resolver a single forwarding slot and holds
		// it, so the query waits for the timeout and is refused.
		limitReached bool
		failWrite    bool // Make writing the response to the client fail.
		// expected series and values, on top of the series that exist from
		// the start: the pre-seeded failover counters and the in-flight
		// gauge, all zero.
		expected map[string]float64
	}{
		{
			description: "A record answered locally",
			query:       new(dns.Msg).SetQuestion("web.", dns.TypeA),
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="udp",qtype="A"}`: 1,
			},
		},
		{
			description: "AAAA record answered locally",
			query:       new(dns.Msg).SetQuestion("web.", dns.TypeAAAA),
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="udp",qtype="AAAA"}`: 1,
			},
		},
		{
			description: "PTR record answered locally",
			query:       new(dns.Msg).SetQuestion("2.0.20.172.in-addr.arpa.", dns.TypePTR),
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="udp",qtype="PTR"}`: 1,
			},
		},
		{
			description: "SRV record answered locally",
			query:       new(dns.Msg).SetQuestion("_http._tcp.web.", dns.TypeSRV),
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="udp",qtype="SRV"}`: 1,
			},
		},
		{
			description: "MX query for a local name answered with implicit MX",
			query:       new(dns.Msg).SetQuestion("web.", dns.TypeMX),
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="udp",qtype="MX"}`: 1,
			},
		},
		{
			description: "A record answered locally over TCP",
			query:       new(dns.Msg).SetQuestion("web.", dns.TypeA),
			proto:       "tcp",
			backend:     localBackend,
			expected: map[string]float64{
				queryMetric + `{outcome="local",proto="tcp",qtype="A"}`: 1,
			},
		},
		{
			description: "unsupported query type is forwarded",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeTXT),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeSuccess)},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="other"}`: 1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:               1,
			},
		},
		{
			description: "name unknown locally, upstream answers",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
			},
		},
		{
			description: "NXDOMAIN from upstream is not a failover",
			query:       new(dns.Msg).SetQuestion("nope.example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeNameError), answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="NXDOMAIN"}`:          1,
			},
		},
		{
			description: "SERVFAIL from first upstream fails over to second",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeServerFailure), answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="SERVFAIL"}`:          1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
				failoverMetric + `{reason="servfail"}`:                      1,
			},
		},
		{
			description: "REFUSED from first upstream fails over to second",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeRefused), answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="REFUSED"}`:           1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
				failoverMetric + `{reason="refused"}`:                       1,
			},
		},
		{
			description: "no usable response from first upstream fails over to second",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerGarbage, answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="error"}`:             1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
				failoverMetric + `{reason="no_response"}`:                   1,
			},
		},
		{
			description: "failures on every upstream count as failovers until the last",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams: []dns.HandlerFunc{
				answerRcode(dns.RcodeServerFailure),
				answerGarbage,
				answerRcode(dns.RcodeServerFailure),
			},
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="SERVFAIL"}`:                2,
				upstreamMetric + `{proto="udp",result="error"}`:                   1,
				failoverMetric + `{reason="servfail"}`:                            1,
				failoverMetric + `{reason="no_response"}`:                         1,
			},
		},
		{
			description: "SERVFAIL from the only upstream is not a failover",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeServerFailure)},
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="SERVFAIL"}`:                1,
			},
		},
		{
			description: "forwarding enabled but no upstream servers",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
			},
		},
		{
			description: "forwarding disabled skips host-loopback upstream servers",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    false,
			upstreams:   []dns.HandlerFunc{answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
			},
		},
		{
			description: "silent upstream server times out",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerNothing},
			timeout:     100 * time.Millisecond,
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="timeout"}`:                 1,
			},
		},
		{
			description: "silent upstream server fails over to the next server",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerNothing, answerA},
			timeout:     100 * time.Millisecond,
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="timeout"}`:           1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
				failoverMetric + `{reason="no_response"}`:                   1,
			},
		},
		{
			description:  "query is refused when the concurrent forwarding limit is reached",
			query:        new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:     true,
			upstreams:    []dns.HandlerFunc{answerA},
			timeout:      100 * time.Millisecond,
			limitReached: true,
			expected: map[string]float64{
				queryMetric + `{outcome="refused",proto="udp",qtype="A"}`: 1,
			},
		},
		{
			description:  "refused query over TCP is labelled with its protocol",
			query:        new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proto:        "tcp",
			proxyDNS:     true,
			upstreams:    []dns.HandlerFunc{answerA},
			timeout:      100 * time.Millisecond,
			limitReached: true,
			expected: map[string]float64{
				queryMetric + `{outcome="refused",proto="tcp",qtype="A"}`: 1,
			},
		},
		{
			description: "forwarding disabled still uses container-namespace upstream servers",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    false,
			upstreams:   []dns.HandlerFunc{answerA},
			containerNS: []bool{true},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
			},
		},
		{
			description: "skipping a host-loopback server is not a failover",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    false,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeServerFailure), answerA},
			containerNS: []bool{false, true},
			expected: map[string]float64{
				queryMetric + `{outcome="forwarded",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="NOERROR"}`:           1,
			},
		},
		{
			description: "SERVFAIL with only a skipped host-loopback server remaining is not a failover",
			query:       new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
			proxyDNS:    false,
			upstreams:   []dns.HandlerFunc{answerRcode(dns.RcodeServerFailure), answerA},
			containerNS: []bool{true, false},
			expected: map[string]float64{
				queryMetric + `{outcome="upstream_failed",proto="udp",qtype="A"}`: 1,
				upstreamMetric + `{proto="udp",result="SERVFAIL"}`:                1,
			},
		},
		{
			description: "single-label name is not forwarded when ndots is set",
			query:       new(dns.Msg).SetQuestion("example.", dns.TypeA),
			backend:     ndotsDNSBackend{},
			proxyDNS:    true,
			upstreams:   []dns.HandlerFunc{answerA},
			expected: map[string]float64{
				queryMetric + `{outcome="ndots",proto="udp",qtype="A"}`: 1,
			},
		},
		{
			description: "internal error while handling the query",
			query:       new(dns.Msg).SetQuestion("_sip._tcp.example.com.", dns.TypeSRV),
			backend:     badSRVDNSBackend{},
			expected: map[string]float64{
				queryMetric + `{outcome="error",proto="udp",qtype="SRV"}`: 1,
			},
		},
		{
			description: "response cannot be written to the client",
			query:       new(dns.Msg).SetQuestion("web.", dns.TypeA),
			backend:     localBackend,
			failWrite:   true,
			expected: map[string]float64{
				queryMetric + `{outcome="write_error",proto="udp",qtype="A"}`: 1,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			var backend DNSBackend = noopDNSBackend{}
			if tc.backend != nil {
				backend = tc.backend
			}
			rsv := NewResolver("", tc.proxyDNS, backend)
			rsv.logger = testLogger(t)
			var reg prometheus.Gatherer
			rsv.metrics, reg = newTestResolverMetrics(t)
			if tc.timeout != 0 {
				rsv.ioTimeout = tc.timeout
			}
			if tc.limitReached {
				rsv.fwdSem = semaphore.NewWeighted(1)
				assert.NilError(t, rsv.fwdSem.Acquire(context.Background(), 1))
			}

			var upstreams []extDNSEntry
			for i, handler := range tc.upstreams {
				upstream, _ := startFakeUpstream(t, handler)
				if i < len(tc.containerNS) && tc.containerNS[i] {
					upstream.HostLoopback = false
				}
				upstreams = append(upstreams, upstream)
			}
			rsv.SetExtServers(upstreams)

			proto := tc.proto
			if proto == "" {
				proto = "udp"
			}
			var w dns.ResponseWriter = &tstwriter{network: proto}
			if tc.failWrite {
				w = &failingWriter{tstwriter{network: proto}}
			}
			rsv.serveDNS(w, tc.query)

			expected := map[string]float64{
				failoverMetric + `{reason="no_response"}`: 0,
				failoverMetric + `{reason="servfail"}`:    0,
				failoverMetric + `{reason="refused"}`:     0,
				inFlightMetric:                            0,
			}
			maps.Copy(expected, tc.expected)
			assert.Check(t, is.DeepEqual(metricsSnapshot(t, reg), expected))
		})
	}
}

func TestResolverMetricsRegistered(t *testing.T) {
	// The default metrics must be registered with the default registry, so
	// that they're served by the daemon's metrics endpoint.
	snapshot := metricsSnapshot(t, prometheus.DefaultGatherer)
	for _, reason := range []string{failoverNoResponse, failoverServFail, failoverRefused} {
		_, ok := snapshot[failoverMetric+`{reason="`+reason+`"}`]
		assert.Check(t, ok, "series for reason %q not found in default registry", reason)
	}
	_, ok := snapshot[inFlightMetric]
	assert.Check(t, ok, "in-flight gauge not found in default registry")
}

func TestQtypeLabel(t *testing.T) {
	testcases := []struct {
		description string
		qtype       uint16
		expected    string
	}{
		{description: "A", qtype: dns.TypeA, expected: "A"},
		{description: "AAAA", qtype: dns.TypeAAAA, expected: "AAAA"},
		{description: "MX", qtype: dns.TypeMX, expected: "MX"},
		{description: "PTR", qtype: dns.TypePTR, expected: "PTR"},
		{description: "SRV", qtype: dns.TypeSRV, expected: "SRV"},
		{description: "TXT is collapsed to other", qtype: dns.TypeTXT, expected: "other"},
		{description: "CNAME is collapsed to other", qtype: dns.TypeCNAME, expected: "other"},
		{description: "HTTPS is collapsed to other", qtype: dns.TypeHTTPS, expected: "other"},
		{description: "ANY is collapsed to other", qtype: dns.TypeANY, expected: "other"},
		{description: "type zero is collapsed to other", qtype: 0, expected: "other"},
		{description: "unassigned type is collapsed to other", qtype: 65535, expected: "other"},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Check(t, is.Equal(qtypeLabel(tc.qtype), tc.expected))
		})
	}
}

// TestQtypeLabelBounded checks that, whatever query type a client sends, the
// "qtype" label takes one of a fixed set of values. This is what keeps the
// number of series bounded.
func TestQtypeLabelBounded(t *testing.T) {
	expected := map[string]bool{"A": true, "AAAA": true, "MX": true, "PTR": true, "SRV": true, "other": true}
	seen := map[string]bool{}
	for qtype := range 1 << 16 {
		seen[qtypeLabel(uint16(qtype))] = true
	}
	assert.Check(t, is.DeepEqual(seen, expected))
}

func TestUpstreamResultLabel(t *testing.T) {
	testcases := []struct {
		description string
		rcode       int
		expected    string
	}{
		{description: "NOERROR", rcode: dns.RcodeSuccess, expected: "NOERROR"},
		{description: "NXDOMAIN", rcode: dns.RcodeNameError, expected: "NXDOMAIN"},
		{description: "SERVFAIL", rcode: dns.RcodeServerFailure, expected: "SERVFAIL"},
		{description: "REFUSED", rcode: dns.RcodeRefused, expected: "REFUSED"},
		{description: "rcode 16 is BADVERS or BADSIG", rcode: dns.RcodeBadVers, expected: "BADSIG"},
		{description: "unknown rcode", rcode: 4095, expected: "other"},
		{description: "negative rcode", rcode: -1, expected: "other"},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Check(t, is.Equal(upstreamResultLabel(tc.rcode), tc.expected))
		})
	}
}

// TestUpstreamResultLabelBounded checks that, whatever response code an
// upstream server returns (including the extended codes carried in EDNS(0)
// OPT records, which go up to 4095), the "result" label takes one of a fixed
// set of values: the names known to the dns package, plus "other".
func TestUpstreamResultLabelBounded(t *testing.T) {
	expected := map[string]bool{upstreamResultOther: true}
	for _, name := range dns.RcodeToString {
		expected[name] = true
	}
	seen := map[string]bool{}
	for rcode := -1; rcode <= 4096; rcode++ {
		seen[upstreamResultLabel(rcode)] = true
	}
	assert.Check(t, is.DeepEqual(seen, expected))
}

func TestHasUsableExtDNS(t *testing.T) {
	host := extDNSEntry{IPStr: "127.0.0.53", HostLoopback: true}
	ctr := extDNSEntry{IPStr: "10.0.0.53"}
	testcases := []struct {
		description string
		entries     []extDNSEntry
		proxyDNS    bool
		expected    bool
	}{
		{description: "nil list", entries: nil, proxyDNS: true, expected: false},
		{description: "empty entries only", entries: []extDNSEntry{{}, {}}, proxyDNS: true, expected: false},
		{description: "host-loopback server with forwarding enabled", entries: []extDNSEntry{host}, proxyDNS: true, expected: true},
		{description: "host-loopback server with forwarding disabled", entries: []extDNSEntry{host}, proxyDNS: false, expected: false},
		{description: "container-namespace server with forwarding disabled", entries: []extDNSEntry{ctr}, proxyDNS: false, expected: true},
		{description: "usable server after an unusable one", entries: []extDNSEntry{host, ctr}, proxyDNS: false, expected: true},
		{description: "server after an empty entry is ignored", entries: []extDNSEntry{{}, ctr}, proxyDNS: true, expected: false},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Check(t, is.Equal(hasUsableExtDNS(tc.entries, tc.proxyDNS), tc.expected))
		})
	}
}
