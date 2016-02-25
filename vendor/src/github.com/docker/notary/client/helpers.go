package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/client/changelist"
	tuf "github.com/docker/notary/tuf"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/store"
	"github.com/docker/notary/tuf/utils"
)

// Use this to initialize remote HTTPStores from the config settings
func getRemoteStore(baseURL, gun string, rt http.RoundTripper) (store.RemoteStore, error) {
	s, err := store.NewHTTPStore(
		baseURL+"/v2/"+gun+"/_trust/tuf/",
		"",
		"json",
		"key",
		rt,
	)
	if err != nil {
		return store.OfflineStore{}, err
	}
	return s, err
}

func applyChangelist(repo *tuf.Repo, cl changelist.Changelist) error {
	it, err := cl.NewIterator()
	if err != nil {
		return err
	}
	index := 0
	for it.HasNext() {
		c, err := it.Next()
		if err != nil {
			return err
		}
		isDel := data.IsDelegation(c.Scope())
		switch {
		case c.Scope() == changelist.ScopeTargets || isDel:
			err = applyTargetsChange(repo, c)
		case c.Scope() == changelist.ScopeRoot:
			err = applyRootChange(repo, c)
		default:
			logrus.Debug("scope not supported: ", c.Scope())
		}
		index++
		if err != nil {
			return err
		}
	}
	logrus.Debugf("applied %d change(s)", index)
	return nil
}

func applyTargetsChange(repo *tuf.Repo, c changelist.Change) error {
	switch c.Type() {
	case changelist.TypeTargetsTarget:
		return changeTargetMeta(repo, c)
	case changelist.TypeTargetsDelegation:
		return changeTargetsDelegation(repo, c)
	default:
		return fmt.Errorf("only target meta and delegations changes supported")
	}
}

func changeTargetsDelegation(repo *tuf.Repo, c changelist.Change) error {
	switch c.Action() {
	case changelist.ActionCreate:
		td := changelist.TufDelegation{}
		err := json.Unmarshal(c.Content(), &td)
		if err != nil {
			return err
		}

		// Try to create brand new role or update one
		// First add the keys, then the paths.  We can only add keys and paths in this scenario
		err = repo.UpdateDelegationKeys(c.Scope(), td.AddKeys, []string{}, td.NewThreshold)
		if err != nil {
			return err
		}
		return repo.UpdateDelegationPaths(c.Scope(), td.AddPaths, []string{}, false)
	case changelist.ActionUpdate:
		td := changelist.TufDelegation{}
		err := json.Unmarshal(c.Content(), &td)
		if err != nil {
			return err
		}
		delgRole, err := repo.GetDelegationRole(c.Scope())
		if err != nil {
			return err
		}

		// We need to translate the keys from canonical ID to TUF ID for compatibility
		canonicalToTUFID := make(map[string]string)
		for tufID, pubKey := range delgRole.Keys {
			canonicalID, err := utils.CanonicalKeyID(pubKey)
			if err != nil {
				return err
			}
			canonicalToTUFID[canonicalID] = tufID
		}

		removeTUFKeyIDs := []string{}
		for _, canonID := range td.RemoveKeys {
			removeTUFKeyIDs = append(removeTUFKeyIDs, canonicalToTUFID[canonID])
		}

		// If we specify the only keys left delete the role, else just delete specified keys
		if strings.Join(delgRole.ListKeyIDs(), ";") == strings.Join(removeTUFKeyIDs, ";") && len(td.AddKeys) == 0 {
			return repo.DeleteDelegation(c.Scope())
		}
		err = repo.UpdateDelegationKeys(c.Scope(), td.AddKeys, removeTUFKeyIDs, td.NewThreshold)
		if err != nil {
			return err
		}
		return repo.UpdateDelegationPaths(c.Scope(), td.AddPaths, td.RemovePaths, td.ClearAllPaths)
	case changelist.ActionDelete:
		return repo.DeleteDelegation(c.Scope())
	default:
		return fmt.Errorf("unsupported action against delegations: %s", c.Action())
	}

}

// applies a function repeatedly, falling back on the parent role, until it no
// longer can
func doWithRoleFallback(role string, doFunc func(string) error) error {
	for role == data.CanonicalTargetsRole || data.IsDelegation(role) {
		err := doFunc(role)
		if err == nil {
			return nil
		}
		if _, ok := err.(data.ErrInvalidRole); !ok {
			return err
		}
		role = path.Dir(role)
	}
	return data.ErrInvalidRole{Role: role}
}

func changeTargetMeta(repo *tuf.Repo, c changelist.Change) error {
	var err error
	switch c.Action() {
	case changelist.ActionCreate:
		logrus.Debug("changelist add: ", c.Path())
		meta := &data.FileMeta{}
		err = json.Unmarshal(c.Content(), meta)
		if err != nil {
			return err
		}
		files := data.Files{c.Path(): *meta}

		err = doWithRoleFallback(c.Scope(), func(role string) error {
			_, e := repo.AddTargets(role, files)
			return e
		})
		if err != nil {
			logrus.Errorf("couldn't add target to %s: %s", c.Scope(), err.Error())
		}

	case changelist.ActionDelete:
		logrus.Debug("changelist remove: ", c.Path())

		err = doWithRoleFallback(c.Scope(), func(role string) error {
			return repo.RemoveTargets(role, c.Path())
		})
		if err != nil {
			logrus.Errorf("couldn't remove target from %s: %s", c.Scope(), err.Error())
		}

	default:
		logrus.Debug("action not yet supported: ", c.Action())
	}
	return err
}

func applyRootChange(repo *tuf.Repo, c changelist.Change) error {
	var err error
	switch c.Type() {
	case changelist.TypeRootRole:
		err = applyRootRoleChange(repo, c)
	default:
		logrus.Debug("type of root change not yet supported: ", c.Type())
	}
	return err // might be nil
}

func applyRootRoleChange(repo *tuf.Repo, c changelist.Change) error {
	switch c.Action() {
	case changelist.ActionCreate:
		// replaces all keys for a role
		d := &changelist.TufRootData{}
		err := json.Unmarshal(c.Content(), d)
		if err != nil {
			return err
		}
		err = repo.ReplaceBaseKeys(d.RoleName, d.Keys...)
		if err != nil {
			return err
		}
	default:
		logrus.Debug("action not yet supported for root: ", c.Action())
	}
	return nil
}

func nearExpiry(r *data.SignedRoot) bool {
	plus6mo := time.Now().AddDate(0, 6, 0)
	return r.Signed.Expires.Before(plus6mo)
}

// Fetches a public key from a remote store, given a gun and role
func getRemoteKey(url, gun, role string, rt http.RoundTripper) (data.PublicKey, error) {
	remote, err := getRemoteStore(url, gun, rt)
	if err != nil {
		return nil, err
	}
	rawPubKey, err := remote.GetKey(role)
	if err != nil {
		return nil, err
	}

	pubKey, err := data.UnmarshalPublicKey(rawPubKey)
	if err != nil {
		return nil, err
	}

	return pubKey, nil
}

// signs and serializes the metadata for a canonical role in a tuf repo to JSON
func serializeCanonicalRole(tufRepo *tuf.Repo, role string) (out []byte, err error) {
	var s *data.Signed
	switch {
	case role == data.CanonicalRootRole:
		s, err = tufRepo.SignRoot(data.DefaultExpires(role))
	case role == data.CanonicalSnapshotRole:
		s, err = tufRepo.SignSnapshot(data.DefaultExpires(role))
	case tufRepo.Targets[role] != nil:
		s, err = tufRepo.SignTargets(
			role, data.DefaultExpires(data.CanonicalTargetsRole))
	default:
		err = fmt.Errorf("%s not supported role to sign on the client", role)
	}

	if err != nil {
		return
	}

	return json.Marshal(s)
}
