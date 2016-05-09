package hcsshim

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
)

// ComputeSystemProperties is a struct describing the returned properties.
type ComputeSystemProperties struct {
	ID                string
	Name              string
	Stopped           bool
	AreUpdatesPending bool
}

// GetComputeSystemProperties gets the properties for the compute system with the given ID.
func GetComputeSystemProperties(id string, flags uint32) (ComputeSystemProperties, error) {
	title := "hcsshim::GetComputeSystemProperties "

	csProps := ComputeSystemProperties{
		Stopped:           false,
		AreUpdatesPending: false,
	}

	logrus.Debugf("Calling proc")
	var buffer *uint16
	err := getComputeSystemProperties(id, flags, &buffer)
	if err != nil {
		err = makeError(err, title, "")
		logrus.Error(err)
		return csProps, err
	}
	propData := convertAndFreeCoTaskMemString(buffer)
	logrus.Debugf(title+" - succeeded output=%s", propData)

	if err = json.Unmarshal([]byte(propData), &csProps); err != nil {
		logrus.Error(err)
		return csProps, err
	}

	return csProps, nil
}
