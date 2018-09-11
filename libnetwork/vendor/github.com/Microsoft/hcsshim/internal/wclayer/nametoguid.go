package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// NameToGuid converts the given string into a GUID using the algorithm in the
// Host Compute Service, ensuring GUIDs generated with the same string are common
// across all clients.
func NameToGuid(name string) (id guid.GUID, err error) {
	title := "hcsshim::NameToGuid "

	err = nameToGuid(name, &id)
	if err != nil {
		err = hcserror.Errorf(err, title, "name=%s", name)
		logrus.Error(err)
		return
	}

	logrus.Debugf(title+"name:%s guid:%s", name, id.String())
	return
}
