package distribution

import (
	"reflect"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/reference"
)

func TestGetRepositoryMountCandidates(t *testing.T) {
	for _, tc := range []struct {
		name          string
		hmacKey       string
		targetRepo    string
		maxCandidates int
		metadata      []metadata.V2Metadata
		candidates    []metadata.V2Metadata
	}{
		{
			name:          "empty metadata",
			targetRepo:    "busybox",
			maxCandidates: -1,
			metadata:      []metadata.V2Metadata{},
			candidates:    []metadata.V2Metadata{},
		},
		{
			name:          "one item not matching",
			targetRepo:    "busybox",
			maxCandidates: -1,
			metadata:      []metadata.V2Metadata{taggedMetadata("key", "dgst", "127.0.0.1/repo")},
			candidates:    []metadata.V2Metadata{},
		},
		{
			name:          "one item matching",
			targetRepo:    "busybox",
			maxCandidates: -1,
			metadata:      []metadata.V2Metadata{taggedMetadata("hash", "1", "hello-world")},
			candidates:    []metadata.V2Metadata{taggedMetadata("hash", "1", "hello-world")},
		},
		{
			name:          "allow missing SourceRepository",
			targetRepo:    "busybox",
			maxCandidates: -1,
			metadata: []metadata.V2Metadata{
				{Digest: digest.Digest("1")},
				{Digest: digest.Digest("3")},
				{Digest: digest.Digest("2")},
			},
			candidates: []metadata.V2Metadata{},
		},
		{
			name:          "handle docker.io",
			targetRepo:    "user/app",
			maxCandidates: -1,
			metadata: []metadata.V2Metadata{
				{Digest: digest.Digest("1"), SourceRepository: "docker.io/user/foo"},
				{Digest: digest.Digest("3"), SourceRepository: "user/bar"},
				{Digest: digest.Digest("2"), SourceRepository: "app"},
			},
			candidates: []metadata.V2Metadata{
				{Digest: digest.Digest("3"), SourceRepository: "user/bar"},
				{Digest: digest.Digest("1"), SourceRepository: "docker.io/user/foo"},
				{Digest: digest.Digest("2"), SourceRepository: "app"},
			},
		},
		{
			name:          "sort more items",
			hmacKey:       "abcd",
			targetRepo:    "127.0.0.1/foo/bar",
			maxCandidates: -1,
			metadata: []metadata.V2Metadata{
				taggedMetadata("hash", "1", "hello-world"),
				taggedMetadata("efgh", "2", "127.0.0.1/hello-world"),
				taggedMetadata("abcd", "3", "busybox"),
				taggedMetadata("hash", "4", "busybox"),
				taggedMetadata("hash", "5", "127.0.0.1/foo"),
				taggedMetadata("hash", "6", "127.0.0.1/bar"),
				taggedMetadata("efgh", "7", "127.0.0.1/foo/bar"),
				taggedMetadata("abcd", "8", "127.0.0.1/xyz"),
				taggedMetadata("hash", "9", "127.0.0.1/foo/app"),
			},
			candidates: []metadata.V2Metadata{
				// first by matching hash
				taggedMetadata("abcd", "8", "127.0.0.1/xyz"),
				// then by longest matching prefix
				taggedMetadata("hash", "9", "127.0.0.1/foo/app"),
				taggedMetadata("hash", "5", "127.0.0.1/foo"),
				// sort the rest of the matching items in reversed order
				taggedMetadata("hash", "6", "127.0.0.1/bar"),
				taggedMetadata("efgh", "2", "127.0.0.1/hello-world"),
			},
		},
		{
			name:          "limit max candidates",
			hmacKey:       "abcd",
			targetRepo:    "user/app",
			maxCandidates: 3,
			metadata: []metadata.V2Metadata{
				taggedMetadata("abcd", "1", "user/app1"),
				taggedMetadata("abcd", "2", "user/app/base"),
				taggedMetadata("hash", "3", "user/app"),
				taggedMetadata("abcd", "4", "127.0.0.1/user/app"),
				taggedMetadata("hash", "5", "user/foo"),
				taggedMetadata("hash", "6", "app/bar"),
			},
			candidates: []metadata.V2Metadata{
				// first by matching hash
				taggedMetadata("abcd", "2", "user/app/base"),
				taggedMetadata("abcd", "1", "user/app1"),
				// then by longest matching prefix
				taggedMetadata("hash", "3", "user/app"),
			},
		},
	} {
		repoInfo, err := reference.ParseNamed(tc.targetRepo)
		if err != nil {
			t.Fatalf("[%s] failed to parse reference name: %v", tc.name, err)
		}
		candidates := getRepositoryMountCandidates(repoInfo, []byte(tc.hmacKey), tc.maxCandidates, tc.metadata)
		if len(candidates) != len(tc.candidates) {
			t.Errorf("[%s] got unexpected number of candidates: %d != %d", tc.name, len(candidates), len(tc.candidates))
		}
		for i := 0; i < len(candidates) && i < len(tc.candidates); i++ {
			if !reflect.DeepEqual(candidates[i], tc.candidates[i]) {
				t.Errorf("[%s] candidate %d does not match expected: %#+v != %#+v", tc.name, i, candidates[i], tc.candidates[i])
			}
		}
		for i := len(candidates); i < len(tc.candidates); i++ {
			t.Errorf("[%s] missing expected candidate at position %d (%#+v)", tc.name, i, tc.candidates[i])
		}
		for i := len(tc.candidates); i < len(candidates); i++ {
			t.Errorf("[%s] got unexpected candidate at position %d (%#+v)", tc.name, i, candidates[i])
		}
	}
}

func taggedMetadata(key string, dgst string, sourceRepo string) metadata.V2Metadata {
	meta := metadata.V2Metadata{
		Digest:           digest.Digest(dgst),
		SourceRepository: sourceRepo,
	}

	meta.HMAC = metadata.ComputeV2MetadataHMAC([]byte(key), &meta)
	return meta
}
