package reference

import (
	"testing"
)

func TestSubstituteReferenceName(t *testing.T) {
	cases := []struct {
		testName        string
		referenceString string
		reposName       string
		expReference    string
		expFailure      bool
	}{
		{
			testName:        "replace untagged refererence with the same name",
			referenceString: "docker/docker",
			reposName:       "docker/docker",
			expReference:    "docker/docker",
		},
		{
			testName:        "replace tagged refererence with the same name",
			referenceString: "docker/docker:tag",
			reposName:       "docker/docker",
			expReference:    "docker/docker:tag",
		},
		{
			testName:        "qualify official repository",
			referenceString: "library/foo:latest",
			reposName:       "index.docker.io/library/foo",
			expReference:    "docker.io/foo:latest",
		},
		{
			testName:        "qualify unofficial repository",
			referenceString: "foo/bar:baz",
			reposName:       "docker.io/foo/bar",
			expReference:    "docker.io/foo/bar:baz",
		},
		{
			testName:        "qualify digested reference unofficial repository",
			referenceString: "docker.io/foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
			reposName:       "localhost:5000/foo/bar",
			expReference:    "localhost:5000/foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
		},
		{
			testName:        "fail due to empty name",
			referenceString: "foo/bar:baz",
			reposName:       "",
			expFailure:      true,
		},
		{
			testName:        "fail due to invalid name",
			referenceString: "foo/bar:baz",
			reposName:       "docker/Docker",
			expFailure:      true,
		},
		{
			testName:        "fail due to invalid name",
			referenceString: "foo/bar:baz",
			reposName:       "docker///docker",
			expFailure:      true,
		},
		{
			testName:        "fail due to tagged replacement",
			referenceString: "foo/bar:baz",
			reposName:       "docker///docker",
			expFailure:      true,
		},
		{
			testName:        "fail due to tagged replacement",
			referenceString: "foo/bar:baz",
			reposName:       "foo/bar:xyz",
			expFailure:      true,
		},
		{
			testName:        "fail due to digested replacement",
			referenceString: "foo/bar:baz",
			reposName:       "foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
			expFailure:      true,
		},
	}

	for _, tc := range cases {
		ref, err := ParseNamed(tc.referenceString)
		if err != nil {
			t.Fatalf("%s: failed to create reference from %q, %v", tc.testName, tc.referenceString, err)
		}
		newRef, err := SubstituteReferenceName(ref, tc.reposName)
		if tc.expFailure && err == nil {
			t.Errorf("%s: got unexpected non-error", tc.testName)
		} else if !tc.expFailure && err != nil {
			t.Errorf("%s: got unexpected error: %v", tc.testName, err)
		} else if !tc.expFailure {
			if newRef.String() != tc.expReference {
				t.Errorf("%s: got unexpected result: %q != %q", tc.testName, ref.String(), tc.expReference)
			}
		}
	}
}

func TestUnqualifyReference(t *testing.T) {
	cases := []struct {
		testName        string
		referenceString string
		expReference    string
	}{
		{
			testName:        "unqualify not qualified",
			referenceString: "docker/docker:tag",
			expReference:    "docker/docker:tag",
		},
		{
			testName:        "unqualify qualified unofficial repository",
			referenceString: "localhost:5000/docker/docker:tag",
			expReference:    "docker/docker:tag",
		},
		{
			testName:        "unqualify qualified official repository",
			referenceString: "docker.io/library/foo:latest",
			expReference:    "foo:latest",
		},
		{
			testName:        "unqualify digested official repository",
			referenceString: "docker.io/foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
			expReference:    "foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
		},
		{
			testName:        "unqualify plain official repository",
			referenceString: "index.docker.io/foo/bar",
			expReference:    "foo/bar",
		},
		{
			testName:        "unqualify plain unofficial repository",
			referenceString: "localhost/foo/bar",
			expReference:    "foo/bar",
		},
		{
			testName:        "unqualify long official repository ",
			referenceString: "docker.io/foo/bar/baz:tagged",
			expReference:    "foo/bar/baz:tagged",
		},
		{
			testName:        "unqualify long unofficial repository ",
			referenceString: "localhost:5000/foo/bar/baz:tagged",
			expReference:    "foo/bar/baz:tagged",
		},
		{
			testName:        "unqualify long unqualified repository ",
			referenceString: "foo/bar/baz:tagged",
			expReference:    "foo/bar/baz:tagged",
		},
	}

	for _, tc := range cases {
		ref, err := ParseNamed(tc.referenceString)
		if err != nil {
			t.Fatalf("%s: failed to create reference from %q, %v", tc.testName, tc.referenceString, err)
		}
		newRef := UnqualifyReference(ref)
		if err != nil {
			t.Errorf("%s: got unexpected error: %v", tc.testName, err)
		} else {
			if newRef.String() != tc.expReference {
				t.Errorf("%s: got unexpected result: %q != %q", ref.String(), newRef.String(), tc.expReference)
			}
		}
	}
}

func TestQualifyUnqualifiedReference(t *testing.T) {
	cases := []struct {
		testName        string
		referenceString string
		indexName       string
		expReference    string
		expFailure      bool
	}{
		{
			testName:        "qualify qualified official repository",
			referenceString: "docker.io/library/docker:tag",
			indexName:       "docker.io",
			expReference:    "docker.io/docker:tag",
		},
		{
			testName:        "qualify qualified unofficial repository",
			referenceString: "localhost/library/docker:tag",
			indexName:       "docker.io",
			expReference:    "localhost/library/docker:tag",
		},
		{
			testName:        "qualify qualified unofficial plain repository",
			referenceString: "localhost/library/docker",
			indexName:       "localhost:5000",
			expReference:    "localhost/library/docker",
		},
		{
			testName:        "qualify unqualified official repository",
			referenceString: "library/docker:tag",
			indexName:       "docker.io",
			expReference:    "docker.io/docker:tag",
		},
		{
			testName:        "qualify unqualified official repository with non-default index",
			referenceString: "library/docker:tag",
			indexName:       "localhost",
			expReference:    "localhost/library/docker:tag",
		},
		{
			testName:        "qualify digested reference",
			referenceString: "foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
			indexName:       "index.docker.io",
			expReference:    "docker.io/foo/bar@sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa",
		},
		{
			testName:        "qualify with no index",
			referenceString: "library/docker:tag",
			indexName:       "",
			expFailure:      true,
		},
		{
			testName:        "fail on invalid index",
			referenceString: "docker/docker",
			indexName:       "docker",
			expFailure:      true,
		},
		{
			testName:        "fail on invalid index 2",
			referenceString: "library/docker:tag",
			indexName:       "-docker.io",
			expFailure:      true,
		},
	}

	for _, tc := range cases {
		ref, err := ParseNamed(tc.referenceString)
		if err != nil {
			t.Fatalf("%s: failed to create reference from %q, %v", tc.testName, tc.referenceString, err)
		}
		newRef, err := QualifyUnqualifiedReference(ref, tc.indexName)
		if tc.expFailure && err == nil {
			t.Errorf("%s: got unexpected non-error", tc.testName)
		} else if !tc.expFailure && err != nil {
			t.Errorf("%s: got unexpected error: %v", tc.testName, err)
		} else if !tc.expFailure {
			if newRef.String() != tc.expReference {
				t.Errorf("%s: got unexpected result: %q != %q", ref.String(), newRef.String(), tc.expReference)
			}
		}
	}
}
