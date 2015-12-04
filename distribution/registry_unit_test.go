package distribution

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
)

func TestTokenPassThru(t *testing.T) {
	authConfig := &types.AuthConfig{
		RegistryToken: "mysecrettoken",
	}
	gotToken := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Authorization"), authConfig.RegistryToken) {
			logrus.Debug("Detected registry token in auth header")
			gotToken = true
		}
		if r.RequestURI == "/v2/" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="foorealm"`)
			w.WriteHeader(401)
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	endpoint := registry.APIEndpoint{
		Mirror:       false,
		URL:          ts.URL,
		Version:      2,
		Official:     false,
		TrimHostname: false,
		TLSConfig:    nil,
		//VersionHeader: "verheader",
		Versions: []auth.APIVersion{
			{
				Type:    "registry",
				Version: "2",
			},
		},
	}
	n, _ := reference.ParseNamed("testremotename")
	repoInfo := &registry.RepositoryInfo{
		Index: &registrytypes.IndexInfo{
			Name:     "testrepo",
			Mirrors:  nil,
			Secure:   false,
			Official: false,
		},
		RemoteName:    n,
		LocalName:     n,
		CanonicalName: n,
		Official:      false,
	}
	imagePullConfig := &ImagePullConfig{
		MetaHeaders: http.Header{},
		AuthConfig:  authConfig,
	}
	puller, err := newPuller(endpoint, repoInfo, imagePullConfig)
	if err != nil {
		t.Fatal(err)
	}
	p := puller.(*v2Puller)
	p.repo, err = NewV2Repository(p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	if err != nil {
		t.Fatal(err)
	}

	logrus.Debug("About to pull")
	// We expect it to fail, since we haven't mock'd the full registry exchange in our handler above
	tag, _ := reference.WithTag(n, "tag_goes_here")
	_ = p.pullV2Repository(context.Background(), tag)

	if !gotToken {
		t.Fatal("Failed to receive registry token")
	}

}
