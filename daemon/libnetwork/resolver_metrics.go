package libnetwork

import (
	gometrics "github.com/docker/go-metrics"
	"github.com/miekg/dns"
)

// Values for the "outcome" label of the query duration metric.
const (
	// queryOutcomeLocal is used when the query was answered from the
	// sandbox's networks.
	queryOutcomeLocal = "local"
	// queryOutcomeNdots is used when a single-label name was not forwarded
	// because the container has a non-zero ndots option.
	queryOutcomeNdots = "ndots"
	// queryOutcomeForwarded is used when an upstream server answered the
	// query, whatever the response code.
	queryOutcomeForwarded = "forwarded"
	// queryOutcomeRefused is used when the query was refused because the
	// limit on concurrent forwarded queries was reached.
	queryOutcomeRefused = "refused"
	// queryOutcomeUpstreamFailed is used when no upstream server answered
	// the query, either because they all failed or because none could be
	// used.
	queryOutcomeUpstreamFailed = "upstream_failed"
	// queryOutcomeError is used when the resolver failed to handle the
	// query.
	queryOutcomeError = "error"
	// queryOutcomeWriteError is used when the response could not be
	// written to the client.
	queryOutcomeWriteError = "write_error"
)

// Values for the "reason" label of the upstream failover counter.
const (
	failoverNoResponse = "no_response"
	failoverServFail   = "servfail"
	failoverRefused    = "refused"
)

// Values for the "result" label of the upstream request duration metric,
// used when there's no response code to report.
const (
	upstreamResultDialError = "dial_error"
	upstreamResultTimeout   = "timeout"
	upstreamResultError     = "error"
	upstreamResultOther     = "other"
)

// resolverDurationBuckets are the histogram buckets for query durations, in
// seconds. Local answers take microseconds, upstream answers take
// milliseconds, and a query can take more than 10 seconds if every upstream
// server times out.
var resolverDurationBuckets = []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// resolverMetrics holds the Prometheus metrics for the embedded DNS resolver.
// The metrics are shared by all resolvers in the daemon; they're not labeled
// by network or container to keep the number of series bounded.
type resolverMetrics struct {
	queryDuration     gometrics.LabeledTimer
	upstreamDuration  gometrics.LabeledTimer
	upstreamFailovers gometrics.LabeledCounter
	upstreamInFlight  gometrics.Gauge
}

var (
	resolverMetricsNS      = gometrics.NewNamespace("libnetwork", "resolver", nil)
	defaultResolverMetrics = newResolverMetrics(resolverMetricsNS)
)

func init() {
	gometrics.Register(resolverMetricsNS)
}

// newResolverMetrics creates the resolver's metrics in namespace ns.
func newResolverMetrics(ns *gometrics.Namespace) *resolverMetrics {
	m := &resolverMetrics{
		queryDuration: ns.NewLabeledTimerWithBuckets("query_duration",
			"Time taken by the embedded DNS resolver to answer a query, in seconds",
			resolverDurationBuckets, "qtype", "proto", "outcome"),
		upstreamDuration: ns.NewLabeledTimerWithBuckets("upstream_request_duration",
			"Time taken by each request to an upstream DNS server, in seconds",
			resolverDurationBuckets, "proto", "result"),
		upstreamFailovers: ns.NewLabeledCounter("upstream_failovers",
			"Number of times a request to an upstream DNS server failed and the query was retried with the next server",
			"reason"),
		upstreamInFlight: ns.NewGauge("upstream_in_flight_requests",
			"Number of DNS queries currently being forwarded to upstream servers. Queries waiting for a slot under the concurrent forwarding limit aren't counted, so the value never exceeds that limit", ""),
	}
	// Failovers are rare and worth alerting on, so make sure the series
	// exist before the first one happens.
	for _, reason := range []string{failoverNoResponse, failoverServFail, failoverRefused} {
		m.upstreamFailovers.WithValues(reason).Inc(0)
	}
	return m
}

// qtypeLabel returns the value of the "qtype" label for a query type. Only
// the types the resolver answers itself get their own value, to bound the
// number of series.
func qtypeLabel(qtype uint16) string {
	switch qtype {
	case dns.TypeA, dns.TypeAAAA, dns.TypeMX, dns.TypePTR, dns.TypeSRV:
		return dns.TypeToString[qtype]
	default:
		return "other"
	}
}

// upstreamResultLabel returns the value of the "result" label for a response
// with the given response code.
func upstreamResultLabel(rcode int) string {
	if s, ok := dns.RcodeToString[rcode]; ok {
		return s
	}
	return upstreamResultOther
}

// hasUsableExtDNS reports whether any of entries would be used to forward a
// query, given the forwarding policy. Entries are in the order they'd be
// tried, and the list ends at the first empty entry.
func hasUsableExtDNS(entries []extDNSEntry, proxyDNS bool) bool {
	for _, e := range entries {
		if e.IPStr == "" {
			return false
		}
		if proxyDNS || !e.HostLoopback {
			return true
		}
	}
	return false
}
