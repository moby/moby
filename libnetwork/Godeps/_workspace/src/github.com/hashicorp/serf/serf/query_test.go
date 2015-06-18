package serf

import (
	"testing"
	"time"
)

func TestDefaultQuery(t *testing.T) {
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	timeout := s1.DefaultQueryTimeout()
	if timeout != s1Config.MemberlistConfig.GossipInterval*time.Duration(s1Config.QueryTimeoutMult) {
		t.Fatalf("bad: %v", timeout)
	}

	params := s1.DefaultQueryParams()
	if params.FilterNodes != nil {
		t.Fatalf("bad: %v", *params)
	}
	if params.FilterTags != nil {
		t.Fatalf("bad: %v", *params)
	}
	if params.RequestAck {
		t.Fatalf("bad: %v", *params)
	}
	if params.Timeout != timeout {
		t.Fatalf("bad: %v", *params)
	}
}

func TestQueryParams_EncodeFilters(t *testing.T) {
	q := &QueryParam{
		FilterNodes: []string{"foo", "bar"},
		FilterTags: map[string]string{
			"role":       "^web",
			"datacenter": "aws$",
		},
	}

	filters, err := q.encodeFilters()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(filters) != 3 {
		t.Fatalf("bad: %v", filters)
	}

	nodeFilt := filters[0]
	if filterType(nodeFilt[0]) != filterNodeType {
		t.Fatalf("bad: %v", nodeFilt)
	}

	tagFilt := filters[1]
	if filterType(tagFilt[0]) != filterTagType {
		t.Fatalf("bad: %v", tagFilt)
	}

	tagFilt = filters[2]
	if filterType(tagFilt[0]) != filterTagType {
		t.Fatalf("bad: %v", tagFilt)
	}
}

func TestSerf_ShouldProcess(t *testing.T) {
	s1Config := testConfig()
	s1Config.NodeName = "zip"
	s1Config.Tags = map[string]string{
		"role":       "webserver",
		"datacenter": "east-aws",
	}
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	// Try a matching query
	q := &QueryParam{
		FilterNodes: []string{"foo", "bar", "zip"},
		FilterTags: map[string]string{
			"role":       "^web",
			"datacenter": "aws$",
		},
	}
	filters, err := q.encodeFilters()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !s1.shouldProcessQuery(filters) {
		t.Fatalf("expected true")
	}

	// Omit node
	q = &QueryParam{
		FilterNodes: []string{"foo", "bar"},
	}
	filters, err = q.encodeFilters()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s1.shouldProcessQuery(filters) {
		t.Fatalf("expected false")
	}

	// Filter on missing tag
	q = &QueryParam{
		FilterTags: map[string]string{
			"other": "cool",
		},
	}
	filters, err = q.encodeFilters()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s1.shouldProcessQuery(filters) {
		t.Fatalf("expected false")
	}

	// Bad tag
	q = &QueryParam{
		FilterTags: map[string]string{
			"role": "db",
		},
	}
	filters, err = q.encodeFilters()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s1.shouldProcessQuery(filters) {
		t.Fatalf("expected false")
	}
}
