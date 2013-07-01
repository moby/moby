package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"time"
	"path"
)

type Session struct {
	root string
	ID string

Created time.Time

	Name string
	AutomaticRelease bool
	Timeout int64

	Path string

	runtime *Runtime
}


type SessionConfig struct {
	AutomaticRelease bool
	Timeout int64
	Name string
}

func (session *Session) When() time.Time {
	return session.Created
}


func (session *Session) jsonPath() string {
	return path.Join(session.root, "config.json")
}


func (session *Session) FromDisk() error {
	data, err := ioutil.ReadFile(session.jsonPath())
	if err != nil {
		return err
	}
	// Load session settings
	if err := json.Unmarshal(data, session); err != nil {
		return err
	}
	return nil
}

func (session *Session) ToDisk() (err error) {
	data, err := json.Marshal(session)
	if err != nil {
		return
	}
	return ioutil.WriteFile(session.jsonPath(), data, 0666)
}

// ShortID returns a shorthand version of the session's id for convenience.
// A collision with other session shorthands is very unlikely, but possible.
// In case of a collision a lookup with Runtime.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length session Id.
func (session *Session) ShortID() string {
	return utils.TruncateID(session.ID)
}


