package api

import (
	"testing"
)

func TestSession_CreateDestroy(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	session := c.Session()

	id, meta, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	meta, err = session.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}
}

func TestSession_CreateRenewDestroy(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	session := c.Session()

	se := &SessionEntry{
		TTL: "10s",
	}

	id, meta, err := session.Create(se, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	renew, meta, err := session.Renew(id, nil)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if renew == nil {
		t.Fatalf("should get session")
	}

	if renew.ID != id {
		t.Fatalf("should have matching id")
	}

	if renew.TTL != "10s" {
		t.Fatalf("should get session with TTL")
	}
}

func TestSession_Info(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}

	if info == nil {
		t.Fatalf("should get session")
	}
	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	if info.ID != id {
		t.Fatalf("bad: %v", info)
	}
	if info.Name != "" {
		t.Fatalf("bad: %v", info)
	}
	if info.Node == "" {
		t.Fatalf("bad: %v", info)
	}
	if len(info.Checks) == 0 {
		t.Fatalf("bad: %v", info)
	}
	if info.LockDelay == 0 {
		t.Fatalf("bad: %v", info)
	}
	if info.Behavior != "release" {
		t.Fatalf("bad: %v", info)
	}
	if info.TTL != "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestSession_Node(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sessions, qm, err := session.Node(info.Node, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("bad: %v", sessions)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}

func TestSession_List(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	sessions, qm, err := session.List(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("bad: %v", sessions)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}
