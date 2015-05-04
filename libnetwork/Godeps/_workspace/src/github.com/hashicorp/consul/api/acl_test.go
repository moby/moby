package api

import (
	"os"
	"testing"
)

// ROOT is a management token for the tests
var CONSUL_ROOT string

func init() {
	CONSUL_ROOT = os.Getenv("CONSUL_ROOT")
}

func TestACL_CreateDestroy(t *testing.T) {
	if CONSUL_ROOT == "" {
		t.SkipNow()
	}
	c, s := makeClient(t)
	defer s.stop()

	c.config.Token = CONSUL_ROOT
	acl := c.ACL()

	ae := ACLEntry{
		Name:  "API test",
		Type:  ACLClientType,
		Rules: `key "" { policy = "deny" }`,
	}

	id, wm, err := acl.Create(&ae, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	ae2, _, err := acl.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ae2.Name != ae.Name || ae2.Type != ae.Type || ae2.Rules != ae.Rules {
		t.Fatalf("Bad: %#v", ae2)
	}

	wm, err = acl.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}
}

func TestACL_CloneDestroy(t *testing.T) {
	if CONSUL_ROOT == "" {
		t.SkipNow()
	}
	c, s := makeClient(t)
	defer s.stop()

	c.config.Token = CONSUL_ROOT
	acl := c.ACL()

	id, wm, err := acl.Clone(CONSUL_ROOT, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	wm, err = acl.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}
}

func TestACL_Info(t *testing.T) {
	if CONSUL_ROOT == "" {
		t.SkipNow()
	}
	c, s := makeClient(t)
	defer s.stop()

	c.config.Token = CONSUL_ROOT
	acl := c.ACL()

	ae, qm, err := acl.Info(CONSUL_ROOT, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}

	if ae == nil || ae.ID != CONSUL_ROOT || ae.Type != ACLManagementType {
		t.Fatalf("bad: %#v", ae)
	}
}

func TestACL_List(t *testing.T) {
	if CONSUL_ROOT == "" {
		t.SkipNow()
	}
	c, s := makeClient(t)
	defer s.stop()

	c.config.Token = CONSUL_ROOT
	acl := c.ACL()

	acls, qm, err := acl.List(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(acls) < 2 {
		t.Fatalf("bad: %v", acls)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}
