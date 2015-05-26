package api

import (
	"strings"
	"testing"
)

func TestAgent_Self(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	name := info["Config"]["NodeName"]
	if name == "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestAgent_Members(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	members, err := agent.Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAgent_Services(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["service:foo"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ServiceAddress(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:    "foo1",
		Port:    8000,
		Address: "192.168.0.42",
	}
	reg2 := &AgentServiceRegistration{
		Name: "foo2",
		Port: 8000,
	}
	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo1"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if _, ok := services["foo2"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	if services["foo1"].Address != "192.168.0.42" {
		t.Fatalf("missing Address field in service foo1: %v", services)
	}
	if services["foo2"].Address != "" {
		t.Fatalf("missing Address field in service foo2: %v", services)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_Services_MultipleChecks(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Checks: AgentServiceChecks{
			&AgentServiceCheck{
				TTL: "15s",
			},
			&AgentServiceCheck{
				TTL: "30s",
			},
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["service:foo:1"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if _, ok := checks["service:foo:2"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
}

func TestAgent_SetTTLStatus(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := agent.WarnTTL("service:foo", "test"); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != "warning" {
		t.Fatalf("Bad: %#v", chk)
	}
	if chk.Output != "test" {
		t.Fatalf("Bad: %#v", chk)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_Checks(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["foo"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_Checks_serviceBound(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check bound to the service
	reg := &AgentCheckRegistration{
		Name:      "redischeck",
		ServiceID: "redis",
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check, ok := checks["redischeck"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if check.ServiceID != "redis" {
		t.Fatalf("missing service association for check: %v", check)
	}
}

func TestAgent_Join(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Join ourself
	addr := info["Config"]["AdvertiseAddr"].(string)
	err = agent.Join(addr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ForceLeave(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	// Eject somebody
	err := agent.ForceLeave("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServiceMaintenance(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Enable maintenance mode
	if err := agent.EnableServiceMaintenance("redis", "broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != "critical" || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableServiceMaintenance("redis"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the critical health check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
	}
}

func TestNodeMaintenance(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	// Enable maintenance mode
	if err := agent.EnableNodeMaintenance("broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != "critical" || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableNodeMaintenance(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
	}
}
