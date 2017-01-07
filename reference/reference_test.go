package reference

import (
	"testing"

	"github.com/opencontainers/go-digest"
)

func TestValidateReferenceName(t *testing.T) {
	validRepoNames := []string{
		"docker/docker",
		"library/debian",
		"debian",
		"docker.io/docker/docker",
		"docker.io/library/debian",
		"docker.io/debian",
		"index.docker.io/docker/docker",
		"index.docker.io/library/debian",
		"index.docker.io/debian",
		"127.0.0.1:5000/docker/docker",
		"127.0.0.1:5000/library/debian",
		"127.0.0.1:5000/debian",
		"thisisthesongthatneverendsitgoesonandonandonthisisthesongthatnev",
	}
	invalidRepoNames := []string{
		"https://github.com/docker/docker",
		"docker/Docker",
		"-docker",
		"-docker/docker",
		"-docker.io/docker/docker",
		"docker///docker",
		"docker.io/docker/Docker",
		"docker.io/docker///docker",
		"1a3f5e7d9c1b3a5f7e9d1c3b5a7f9e1d3c5b7a9f1e3d5d7c9b1a3f5e7d9c1b3a",
		"docker.io/1a3f5e7d9c1b3a5f7e9d1c3b5a7f9e1d3c5b7a9f1e3d5d7c9b1a3f5e7d9c1b3a",
	}

	for _, name := range invalidRepoNames {
		_, err := ParseNamed(name)
		if err == nil {
			t.Fatalf("Expected invalid repo name for %q", name)
		}
	}

	for _, name := range validRepoNames {
		_, err := ParseNamed(name)
		if err != nil {
			t.Fatalf("Error parsing repo name %s, got: %q", name, err)
		}
	}
}

func TestValidateRemoteName(t *testing.T) {
	validRepositoryNames := []string{
		// Sanity check.
		"docker/docker",

		// Allow 64-character non-hexadecimal names (hexadecimal names are forbidden).
		"thisisthesongthatneverendsitgoesonandonandonthisisthesongthatnev",

		// Allow embedded hyphens.
		"docker-rules/docker",

		// Allow multiple hyphens as well.
		"docker---rules/docker",

		//Username doc and image name docker being tested.
		"doc/docker",

		// single character names are now allowed.
		"d/docker",
		"jess/t",

		// Consecutive underscores.
		"dock__er/docker",
	}
	for _, repositoryName := range validRepositoryNames {
		_, err := ParseNamed(repositoryName)
		if err != nil {
			t.Errorf("Repository name should be valid: %v. Error: %v", repositoryName, err)
		}
	}

	invalidRepositoryNames := []string{
		// Disallow capital letters.
		"docker/Docker",

		// Only allow one slash.
		"docker///docker",

		// Disallow 64-character hexadecimal.
		"1a3f5e7d9c1b3a5f7e9d1c3b5a7f9e1d3c5b7a9f1e3d5d7c9b1a3f5e7d9c1b3a",

		// Disallow leading and trailing hyphens in namespace.
		"-docker/docker",
		"docker-/docker",
		"-docker-/docker",

		// Don't allow underscores everywhere (as opposed to hyphens).
		"____/____",

		"_docker/_docker",

		// Disallow consecutive periods.
		"dock..er/docker",
		"dock_.er/docker",
		"dock-.er/docker",

		// No repository.
		"docker/",

		//namespace too long
		"this_is_not_a_valid_namespace_because_its_lenth_is_greater_than_255_this_is_not_a_valid_namespace_because_its_lenth_is_greater_than_255_this_is_not_a_valid_namespace_because_its_lenth_is_greater_than_255_this_is_not_a_valid_namespace_because_its_lenth_is_greater_than_255/docker",
	}
	for _, repositoryName := range invalidRepositoryNames {
		if _, err := ParseNamed(repositoryName); err == nil {
			t.Errorf("Repository name should be invalid: %v", repositoryName)
		}
	}
}

func TestParseRepositoryInfo(t *testing.T) {
	type tcase struct {
		RemoteName, NormalizedName, FullName, AmbiguousName, Hostname string
	}

	tcases := []tcase{
		{
			RemoteName:     "fooo/bar",
			NormalizedName: "fooo/bar",
			FullName:       "docker.io/fooo/bar",
			AmbiguousName:  "index.docker.io/fooo/bar",
			Hostname:       "docker.io",
		},
		{
			RemoteName:     "library/ubuntu",
			NormalizedName: "ubuntu",
			FullName:       "docker.io/library/ubuntu",
			AmbiguousName:  "library/ubuntu",
			Hostname:       "docker.io",
		},
		{
			RemoteName:     "nonlibrary/ubuntu",
			NormalizedName: "nonlibrary/ubuntu",
			FullName:       "docker.io/nonlibrary/ubuntu",
			AmbiguousName:  "",
			Hostname:       "docker.io",
		},
		{
			RemoteName:     "other/library",
			NormalizedName: "other/library",
			FullName:       "docker.io/other/library",
			AmbiguousName:  "",
			Hostname:       "docker.io",
		},
		{
			RemoteName:     "private/moonbase",
			NormalizedName: "127.0.0.1:8000/private/moonbase",
			FullName:       "127.0.0.1:8000/private/moonbase",
			AmbiguousName:  "",
			Hostname:       "127.0.0.1:8000",
		},
		{
			RemoteName:     "privatebase",
			NormalizedName: "127.0.0.1:8000/privatebase",
			FullName:       "127.0.0.1:8000/privatebase",
			AmbiguousName:  "",
			Hostname:       "127.0.0.1:8000",
		},
		{
			RemoteName:     "private/moonbase",
			NormalizedName: "example.com/private/moonbase",
			FullName:       "example.com/private/moonbase",
			AmbiguousName:  "",
			Hostname:       "example.com",
		},
		{
			RemoteName:     "privatebase",
			NormalizedName: "example.com/privatebase",
			FullName:       "example.com/privatebase",
			AmbiguousName:  "",
			Hostname:       "example.com",
		},
		{
			RemoteName:     "private/moonbase",
			NormalizedName: "example.com:8000/private/moonbase",
			FullName:       "example.com:8000/private/moonbase",
			AmbiguousName:  "",
			Hostname:       "example.com:8000",
		},
		{
			RemoteName:     "privatebasee",
			NormalizedName: "example.com:8000/privatebasee",
			FullName:       "example.com:8000/privatebasee",
			AmbiguousName:  "",
			Hostname:       "example.com:8000",
		},
		{
			RemoteName:     "library/ubuntu-12.04-base",
			NormalizedName: "ubuntu-12.04-base",
			FullName:       "docker.io/library/ubuntu-12.04-base",
			AmbiguousName:  "index.docker.io/library/ubuntu-12.04-base",
			Hostname:       "docker.io",
		},
	}

	for _, tcase := range tcases {
		refStrings := []string{tcase.NormalizedName, tcase.FullName}
		if tcase.AmbiguousName != "" {
			refStrings = append(refStrings, tcase.AmbiguousName)
		}

		var refs []Named
		for _, r := range refStrings {
			named, err := ParseNamed(r)
			if err != nil {
				t.Fatal(err)
			}
			refs = append(refs, named)
			named, err = WithName(r)
			if err != nil {
				t.Fatal(err)
			}
			refs = append(refs, named)
		}

		for _, r := range refs {
			if expected, actual := tcase.NormalizedName, r.Name(); expected != actual {
				t.Fatalf("Invalid normalized reference for %q. Expected %q, got %q", r, expected, actual)
			}
			if expected, actual := tcase.FullName, r.FullName(); expected != actual {
				t.Fatalf("Invalid fullName for %q. Expected %q, got %q", r, expected, actual)
			}
			if expected, actual := tcase.Hostname, r.Hostname(); expected != actual {
				t.Fatalf("Invalid hostname for %q. Expected %q, got %q", r, expected, actual)
			}
			if expected, actual := tcase.RemoteName, r.RemoteName(); expected != actual {
				t.Fatalf("Invalid remoteName for %q. Expected %q, got %q", r, expected, actual)
			}

		}
	}
}

func TestParseReferenceWithTagAndDigest(t *testing.T) {
	ref, err := ParseNamed("busybox:latest@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")
	if err != nil {
		t.Fatal(err)
	}
	if _, isTagged := ref.(NamedTagged); isTagged {
		t.Fatalf("Reference from %q should not support tag", ref)
	}
	if _, isCanonical := ref.(Canonical); !isCanonical {
		t.Fatalf("Reference from %q should not support digest", ref)
	}
	if expected, actual := "busybox@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa", ref.String(); actual != expected {
		t.Fatalf("Invalid parsed reference for %q: expected %q, got %q", ref, expected, actual)
	}
}

func TestInvalidReferenceComponents(t *testing.T) {
	if _, err := WithName("-foo"); err == nil {
		t.Fatal("Expected WithName to detect invalid name")
	}
	ref, err := WithName("busybox")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WithTag(ref, "-foo"); err == nil {
		t.Fatal("Expected WithName to detect invalid tag")
	}
	if _, err := WithDigest(ref, digest.Digest("foo")); err == nil {
		t.Fatal("Expected WithName to detect invalid digest")
	}
}
