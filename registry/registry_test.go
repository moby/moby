package registry // import "github.com/docker/docker/registry"

import (
	"testing"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseRepositoryInfo(t *testing.T) {
	type staticRepositoryInfo struct {
		Index         *registry.IndexInfo
		RemoteName    string
		CanonicalName string
		LocalName     string
		Official      bool
	}

	expectedRepoInfos := map[string]staticRepositoryInfo{
		"fooo/bar": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "fooo/bar",
			LocalName:     "fooo/bar",
			CanonicalName: "docker.io/fooo/bar",
			Official:      false,
		},
		"library/ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu",
			LocalName:     "ubuntu",
			CanonicalName: "docker.io/library/ubuntu",
			Official:      true,
		},
		"nonlibrary/ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "nonlibrary/ubuntu",
			LocalName:     "nonlibrary/ubuntu",
			CanonicalName: "docker.io/nonlibrary/ubuntu",
			Official:      false,
		},
		"ubuntu": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu",
			LocalName:     "ubuntu",
			CanonicalName: "docker.io/library/ubuntu",
			Official:      true,
		},
		"other/library": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "other/library",
			LocalName:     "other/library",
			CanonicalName: "docker.io/other/library",
			Official:      false,
		},
		"127.0.0.1:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "127.0.0.1:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "127.0.0.1:8000/private/moonbase",
			CanonicalName: "127.0.0.1:8000/private/moonbase",
			Official:      false,
		},
		"127.0.0.1:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "127.0.0.1:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "127.0.0.1:8000/privatebase",
			CanonicalName: "127.0.0.1:8000/privatebase",
			Official:      false,
		},
		"localhost:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "localhost:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "localhost:8000/private/moonbase",
			CanonicalName: "localhost:8000/private/moonbase",
			Official:      false,
		},
		"localhost:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "localhost:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "localhost:8000/privatebase",
			CanonicalName: "localhost:8000/privatebase",
			Official:      false,
		},
		"example.com/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "example.com",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "example.com/private/moonbase",
			CanonicalName: "example.com/private/moonbase",
			Official:      false,
		},
		"example.com/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "example.com",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "example.com/privatebase",
			CanonicalName: "example.com/privatebase",
			Official:      false,
		},
		"example.com:8000/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "example.com:8000",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "example.com:8000/private/moonbase",
			CanonicalName: "example.com:8000/private/moonbase",
			Official:      false,
		},
		"example.com:8000/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "example.com:8000",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "example.com:8000/privatebase",
			CanonicalName: "example.com:8000/privatebase",
			Official:      false,
		},
		"localhost/private/moonbase": {
			Index: &registry.IndexInfo{
				Name:     "localhost",
				Official: false,
			},
			RemoteName:    "private/moonbase",
			LocalName:     "localhost/private/moonbase",
			CanonicalName: "localhost/private/moonbase",
			Official:      false,
		},
		"localhost/privatebase": {
			Index: &registry.IndexInfo{
				Name:     "localhost",
				Official: false,
			},
			RemoteName:    "privatebase",
			LocalName:     "localhost/privatebase",
			CanonicalName: "localhost/privatebase",
			Official:      false,
		},
		IndexName + "/public/moonbase": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "public/moonbase",
			LocalName:     "public/moonbase",
			CanonicalName: "docker.io/public/moonbase",
			Official:      false,
		},
		"index." + IndexName + "/public/moonbase": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "public/moonbase",
			LocalName:     "public/moonbase",
			CanonicalName: "docker.io/public/moonbase",
			Official:      false,
		},
		"ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
		IndexName + "/ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
		"index." + IndexName + "/ubuntu-12.04-base": {
			Index: &registry.IndexInfo{
				Name:     IndexName,
				Official: true,
			},
			RemoteName:    "library/ubuntu-12.04-base",
			LocalName:     "ubuntu-12.04-base",
			CanonicalName: "docker.io/library/ubuntu-12.04-base",
			Official:      true,
		},
	}

	for reposName, expectedRepoInfo := range expectedRepoInfos {
		named, err := reference.ParseNormalizedNamed(reposName)
		if err != nil {
			t.Error(err)
		}

		repoInfo, err := ParseRepositoryInfo(named)
		if err != nil {
			t.Error(err)
		} else {
			assert.Check(t, is.Equal(repoInfo.Index.Name, expectedRepoInfo.Index.Name), reposName)
			assert.Check(t, is.Equal(reference.Path(repoInfo.Name), expectedRepoInfo.RemoteName), reposName)
			assert.Check(t, is.Equal(reference.FamiliarName(repoInfo.Name), expectedRepoInfo.LocalName), reposName)
			assert.Check(t, is.Equal(repoInfo.Name.Name(), expectedRepoInfo.CanonicalName), reposName)
			assert.Check(t, is.Equal(repoInfo.Index.Official, expectedRepoInfo.Index.Official), reposName)
			assert.Check(t, is.Equal(repoInfo.Official, expectedRepoInfo.Official), reposName)
		}
	}
}

func TestNewIndexInfo(t *testing.T) {
	testIndexInfo := func(config *serviceConfig, expectedIndexInfos map[string]*registry.IndexInfo) {
		for indexName, expectedIndexInfo := range expectedIndexInfos {
			index, err := newIndexInfo(config, indexName)
			if err != nil {
				t.Fatal(err)
			} else {
				assert.Check(t, is.Equal(index.Name, expectedIndexInfo.Name), indexName+" name")
				assert.Check(t, is.Equal(index.Official, expectedIndexInfo.Official), indexName+" is official")
				assert.Check(t, is.Equal(index.Secure, expectedIndexInfo.Secure), indexName+" is secure")
				assert.Check(t, is.Equal(len(index.Mirrors), len(expectedIndexInfo.Mirrors)), indexName+" mirrors")
			}
		}
	}

	config := emptyServiceConfig
	var noMirrors []string
	expectedIndexInfos := map[string]*registry.IndexInfo{
		IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"index." + IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)

	publicMirrors := []string{"http://mirror1.local", "http://mirror2.local"}
	var err error
	config, err = makeServiceConfig(publicMirrors, []string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}

	expectedIndexInfos = map[string]*registry.IndexInfo{
		IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  publicMirrors,
		},
		"index." + IndexName: {
			Name:     IndexName,
			Official: true,
			Secure:   true,
			Mirrors:  publicMirrors,
		},
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"example.com:5000": {
			Name:     "example.com:5000",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
		"127.0.0.1": {
			Name:     "127.0.0.1",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"other.com": {
			Name:     "other.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)

	config, err = makeServiceConfig(nil, []string{"192.0.2.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	expectedIndexInfos = map[string]*registry.IndexInfo{
		"example.com": {
			Name:     "example.com",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"example.com:5000": {
			Name:     "example.com:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1": {
			Name:     "127.0.0.1",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"127.0.0.1:5000": {
			Name:     "127.0.0.1:5000",
			Official: false,
			Secure:   false,
			Mirrors:  noMirrors,
		},
		"other.com": {
			Name:     "other.com",
			Official: false,
			Secure:   true,
			Mirrors:  noMirrors,
		},
	}
	testIndexInfo(config, expectedIndexInfos)
}

func TestMirrorEndpointLookup(t *testing.T) {
	containsMirror := func(endpoints []APIEndpoint) bool {
		for _, pe := range endpoints {
			if pe.URL.Host == "my.mirror" {
				return true
			}
		}
		return false
	}
	cfg, err := makeServiceConfig([]string{"https://my.mirror"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := Service{config: cfg}

	imageName, err := reference.WithName(IndexName + "/test/image")
	if err != nil {
		t.Error(err)
	}
	pushAPIEndpoints, err := s.LookupPushEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if containsMirror(pushAPIEndpoints) {
		t.Fatal("Push endpoint should not contain mirror")
	}

	pullAPIEndpoints, err := s.LookupPullEndpoints(reference.Domain(imageName))
	if err != nil {
		t.Fatal(err)
	}
	if !containsMirror(pullAPIEndpoints) {
		t.Fatal("Pull endpoint should contain mirror")
	}
}

func TestAllowNondistributableArtifacts(t *testing.T) {
	tests := []struct {
		addr       string
		registries []string
		expected   bool
	}{
		{IndexName, nil, false},
		{"example.com", []string{}, false},
		{"example.com", []string{"example.com"}, true},
		{"localhost", []string{"localhost:5000"}, false},
		{"localhost:5000", []string{"localhost:5000"}, true},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"127.0.0.1:5000"}, true},
		{"localhost", nil, false},
		{"localhost:5000", nil, false},
		{"127.0.0.1", nil, false},
		{"localhost", []string{"example.com"}, false},
		{"127.0.0.1", []string{"example.com"}, false},
		{"example.com", nil, false},
		{"example.com", []string{"example.com"}, true},
		{"127.0.0.1", []string{"example.com"}, false},
		{"127.0.0.1:5000", []string{"example.com"}, false},
		{"example.com:5000", []string{"192.0.2.0/24"}, true},
		{"example.com", []string{"192.0.2.0/24"}, true},
		{"example.com:5000", []string{"192.0.2.42/8"}, true},
		{"127.0.0.1:5000", []string{"127.0.0.0/8"}, true},
		{"192.0.2.42:5000", []string{"192.0.2.0/24"}, true},
		{"invalid.example.com", []string{"192.0.2.0/24"}, false},
		{"invalid.example.com", []string{"invalid.example.com"}, true},
		{"invalid.example.com:5000", []string{"invalid.example.com"}, false},
		{"invalid.example.com:5000", []string{"invalid.example.com:5000"}, true},
	}
	for _, tt := range tests {
		config, err := newServiceConfig(ServiceOptions{
			AllowNondistributableArtifacts: tt.registries,
		})
		if err != nil {
			t.Error(err)
		}
		if v := config.allowNondistributableArtifacts(tt.addr); v != tt.expected {
			t.Errorf("allowNondistributableArtifacts failed for %q %v, expected %v got %v", tt.addr, tt.registries, tt.expected, v)
		}
	}
}

func TestIsSecureIndex(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		insecure []string
		expected bool
	}{
		{
			name:     "official registry",
			addr:     IndexName,
			expected: true,
		},
		{
			name:     "registry without insecure registries",
			addr:     "example.com",
			expected: true,
		},
		{
			name:     "registry configured as insecure",
			addr:     "example.com",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "localhost with different configured port",
			addr:     "localhost",
			insecure: []string{"localhost:5000"},
			expected: false,
		},
		{
			name:     "localhost with configured port",
			addr:     "localhost:5000",
			insecure: []string{"localhost:5000"},
			expected: false,
		},
		{
			name:     "localhost with unrelated insecure registry",
			addr:     "localhost",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with configured port",
			addr:     "127.0.0.1:5000",
			insecure: []string{"127.0.0.1:5000"},
			expected: false,
		},
		{
			name:     "localhost is insecure by default",
			addr:     "localhost",
			expected: false,
		},
		{
			name:     "localhost with port is insecure by default",
			addr:     "localhost:5000",
			expected: false,
		},
		{
			name:     "loopback address is insecure by default",
			addr:     "127.0.0.1",
			expected: false,
		},
		{
			name:     "localhost with unrelated insecure registry duplicate",
			addr:     "localhost",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with unrelated insecure registry",
			addr:     "127.0.0.1",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "registry is secure by default",
			addr:     "example.com",
			expected: true,
		},
		{
			name:     "registry configured as insecure duplicate",
			addr:     "example.com",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with unrelated insecure registry duplicate",
			addr:     "127.0.0.1",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "loopback address with port and unrelated insecure registry",
			addr:     "127.0.0.1:5000",
			insecure: []string{"example.com"},
			expected: false,
		},
		{
			name:     "registry with port matching insecure CIDR",
			addr:     "example.com:5000",
			insecure: []string{"192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry matching insecure CIDR",
			addr:     "example.com",
			insecure: []string{"192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry matching masked insecure CIDR",
			addr:     "example.com:5000",
			insecure: []string{"192.0.2.42/24"},
			expected: false,
		},
		{
			name:     "loopback address matching insecure CIDR",
			addr:     "127.0.0.1:5000",
			insecure: []string{"127.0.0.0/8"},
			expected: false,
		},
		{
			name:     "IP address matching masked insecure CIDR",
			addr:     "192.0.2.42:5000",
			insecure: []string{"192.0.2.1/24"},
			expected: false,
		},
		{
			name:     "unresolvable registry does not match insecure CIDR",
			addr:     "invalid.example.com",
			insecure: []string{"192.0.2.0/24"},
			expected: true,
		},
		{
			name:     "unresolvable registry configured as insecure",
			addr:     "invalid.example.com",
			insecure: []string{"invalid.example.com"},
			expected: false,
		},
		{
			name:     "unresolvable registry with port does not match hostname",
			addr:     "invalid.example.com:5000",
			insecure: []string{"invalid.example.com"},
			expected: true,
		},
		{
			name:     "unresolvable registry with port configured as insecure",
			addr:     "invalid.example.com:5000",
			insecure: []string{"invalid.example.com:5000"},
			expected: false,
		},
		{
			name:     "all resolved addresses in insecure subnets are insecure",
			addr:     "loopback.example",
			insecure: []string{"127.0.0.0/8", "::1/128"},
			expected: false,
		},
		{
			name:     "resolved addresses may match different insecure subnets",
			addr:     "mixed.example",
			insecure: []string{"127.0.0.0/8", "192.0.2.0/24"},
			expected: false,
		},
		{
			name:     "registry is secure if any resolved address is outside insecure subnets",
			addr:     "mixed.example",
			insecure: []string{"127.0.0.0/8"},
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := newServiceConfig(ServiceOptions{
				InsecureRegistries: tc.insecure,
			})
			assert.NilError(t, err)

			sec := config.isSecureIndex(tc.addr)
			assert.Equal(t, sec, tc.expected)
		})
	}
}
