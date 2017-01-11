package daemon

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/graphdb"
)

// migrateLegacySqliteLinks migrates sqlite links to use links from HostConfig
// when sqlite links were used, hostConfig.Links was set to nil
func (daemon *Daemon) migrateLegacySqliteLinks(db *graphdb.Database, container *container.Container) error {
	// if links is populated (or an empty slice), then this isn't using sqlite links and can be skipped
	if container.HostConfig == nil || container.HostConfig.Links != nil {
		return nil
	}

	logrus.Debugf("migrating legacy sqlite link info for container: %s", container.ID)

	fullName := container.Name
	if fullName[0] != '/' {
		fullName = "/" + fullName
	}

	// don't use a nil slice, this ensures that the check above will skip once the migration has completed
	links := []string{}
	children, err := db.Children(fullName, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "Cannot find child for") {
			return err
		}
		// else continue... it's ok if we didn't find any children, it'll just be nil and we can continue the migration
	}

	for _, child := range children {
		c, err := daemon.GetContainer(child.Entity.ID())
		if err != nil {
			return err
		}

		links = append(links, c.Name+":"+child.Edge.Name)
	}

	container.HostConfig.Links = links
	return container.WriteHostConfig()
}

// sqliteMigration performs the link graph DB migration.
func (daemon *Daemon) sqliteMigration(containers map[string]*container.Container) error {
	// migrate any legacy links from sqlite
	linkdbFile := filepath.Join(daemon.root, "linkgraph.db")
	var (
		legacyLinkDB *graphdb.Database
		err          error
	)

	legacyLinkDB, err = graphdb.NewSqliteConn(linkdbFile)
	if err != nil {
		return fmt.Errorf("error connecting to legacy link graph DB %s, container links may be lost: %v", linkdbFile, err)
	}
	defer legacyLinkDB.Close()

	for _, c := range containers {
		if err := daemon.migrateLegacySqliteLinks(legacyLinkDB, c); err != nil {
			return err
		}
	}
	return nil
}
