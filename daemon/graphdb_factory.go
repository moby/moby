package daemon

import (
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/graphdb"
)

// A helper factory of link graph for daemon initialization.
// It's not thread safe because there's no concurrent calls in daemon initialization.
type graphdbFactory struct {
	dbs    map[string]*graphdb.Database
	daemon *Daemon
}

func newGraphdbFactory(daemon *Daemon) *graphdbFactory {
	return &graphdbFactory{
		daemon: daemon,
		dbs:    make(map[string]*graphdb.Database),
	}
}

func (f *graphdbFactory) get(driverName string) (*graphdb.Database, error) {
	if db, ok := f.dbs[driverName]; ok {
		return db, nil
	}
	var graphdbPath string
	if driverName == "legacy" {
		graphdbPath = filepath.Join(f.daemon.root, "linkgraph.db")
	} else {
		graphdbPath = filepath.Join(f.daemon.root, "linkgraph-"+driverName+".db")
	}
	graph, err := graphdb.NewSqliteConn(graphdbPath)
	if err != nil {
		return nil, err
	}
	f.dbs[driverName] = graph
	return graph, nil
}

func (f *graphdbFactory) migrate() error {
	legacyGraphdbPath := filepath.Join(f.daemon.root, "linkgraph.db")
	if _, err := os.Stat(legacyGraphdbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	legacyGraphdb, err := f.get("legacy")
	if err != nil {
		return err
	}

	for p, e := range legacyGraphdb.List("/", -1) {
		cid := e.ID()
		container := &Container{}
		container.ID = cid
		container.root = f.daemon.containerRoot(cid)
		if err := container.loadConfig(); err != nil {
			logrus.Warnf("failed to migrate the linkgraph record for container %s: %v", cid, err)
			continue
		}

		graph, err := f.get(container.Driver)
		if err == nil {
			graph.Set(p, cid)
		}
	}
	return os.Remove(legacyGraphdbPath)
}
