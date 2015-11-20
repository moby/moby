package graph

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/docker/docker/daemon/events"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/vfs" // import the vfs driver so it is used in the tests
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

const (
	testLocalImageName      = "myapp"
	testLocalImageID        = "1a2d3c4d4e5fa2d2a21acea242a5e2345d3aefc3e7dfa2a2a2a21a2a2ad2d234"
	testLocalImageIDShort   = "1a2d3c4d4e5f"
	testPrivateIndexName    = "127.0.0.1:8000"
	testPrivateRemoteName   = "privateapp"
	testPrivateImageName    = testPrivateIndexName + "/" + testPrivateRemoteName
	testPrivateImageID      = "5bc255f8699e4ee89ac4469266c3d11515da88fdcbde45d7b069b636ff4efd81"
	testPrivateImageIDShort = "5bc255f8699e"
	testPrivateImageDigest  = "sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb"
)

func fakeTar() (io.Reader, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)

		// Leaving these fields blank requires root privileges
		hdr.Uid = uid
		hdr.Gid = gid

		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

func mkTestTagStore(root string, t *testing.T) *TagStore {
	driver, err := graphdriver.New(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewGraph(root, driver, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tagCfg := &TagStoreConfig{
		Graph:  graph,
		Events: events.New(),
	}
	store, err := NewTagStore(path.Join(root, "tags"), tagCfg)
	if err != nil {
		t.Fatal(err)
	}
	localArchive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img := &image.Image{ID: testLocalImageID}
	if err := graph.Register(v1Descriptor{img}, localArchive); err != nil {
		t.Fatal(err)
	}
	if err := store.Tag(testLocalImageName, "", testLocalImageID, false, true); err != nil {
		t.Fatal(err)
	}
	privateArchive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img = &image.Image{ID: testPrivateImageID}
	if err := graph.Register(v1Descriptor{img}, privateArchive); err != nil {
		t.Fatal(err)
	}
	if err := store.Tag(testPrivateImageName, "", testPrivateImageID, false, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDigest(testPrivateImageName, testPrivateImageDigest, testPrivateImageID, true); err != nil {
		t.Fatal(err)
	}
	return store
}

func imageCount(s *TagStore) int {
	cnt := 0
	for _, repo := range s.Repositories {
		cnt += len(repo)
	}
	return cnt
}

func logStoreContent(t *testing.T, s *TagStore, caseNumber int) {
	prefix := ""
	if caseNumber >= 0 {
		prefix = fmt.Sprintf("[case#%d] ", caseNumber)
	}
	t.Logf("%sstore.Repositories content:", prefix)
	for name, repo := range s.Repositories {
		t.Logf("%s  %s :", prefix, name)
		for tag, id := range repo {
			t.Logf("%s    %s : %s", prefix, tag, id)
		}
	}
}

func TestLookupImage(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)
	defer store.graph.driver.Cleanup()

	localLookups := []string{
		testLocalImageID,
		testLocalImageIDShort,
		testLocalImageName + ":" + testLocalImageID,
		testLocalImageName + ":" + testLocalImageIDShort,
		testLocalImageName,
		testLocalImageName + ":" + tags.DefaultTag,
	}

	privateLookups := []string{
		testPrivateImageID,
		testPrivateImageIDShort,
		testPrivateImageName + ":" + testPrivateImageID,
		testPrivateImageName + ":" + testPrivateImageIDShort,
		testPrivateImageName,
		testPrivateImageName + ":" + tags.DefaultTag,
	}

	invalidLookups := []string{
		testLocalImageName + ":" + "fail",
		"docker.io/" + testPrivateRemoteName,
		testPrivateIndexName + "/" + testLocalImageName,
		"fail:fail",
		// these should fail, because testLocalImageName isn't fully qualified
		"docker.io/" + testLocalImageName,
		"docker.io/" + testLocalImageName + ":" + tags.DefaultTag,
		"index.docker.io/" + testLocalImageName,
		"index.docker.io/" + testLocalImageName + ":" + tags.DefaultTag,
		"library/" + testLocalImageName,
		"library/" + testLocalImageName + ":" + tags.DefaultTag,
		"docker.io/library/" + testLocalImageName,
		"docker.io/library/" + testLocalImageName + ":" + tags.DefaultTag,
		"index.docker.io/library/" + testLocalImageName,
		"index.docker.io/library/" + testLocalImageName + ":" + tags.DefaultTag,
		// these should fail, because stored private image is prefixed with
		// non-default registry
		testPrivateRemoteName + ":" + testPrivateImageID,
		testPrivateRemoteName + ":" + testPrivateImageIDShort,
		testPrivateRemoteName,
		testPrivateRemoteName + ":" + tags.DefaultTag,
	}

	digestLookups := []string{
		testPrivateImageName + "@" + testPrivateImageDigest,
	}

	runCases := func(imageID string, cases []string, valid bool) {
		for _, name := range cases {
			if valid {
				if img, err := store.LookupImage(name); err != nil {
					t.Errorf("Error looking up %s: %s", name, err)
				} else if img == nil {
					t.Errorf("Expected 1 image, none found: %s", name)
				} else if imageID != "" && img.ID != imageID {
					t.Errorf("Expected ID '%s' found '%s'", imageID, img.ID)
				}
			} else {
				if img, err := store.LookupImage(name); err == nil {
					if img == nil {
						t.Errorf("Expected error, none found: %s", name)
					} else {
						t.Errorf("Expected error, found image %q for %s", img.ID, name)
					}
				} else if img != nil {
					t.Errorf("Expected 0 image, 1 found: %s", name)
				}
			}
		}
	}

	runCases(testLocalImageID, localLookups, true)
	runCases(testPrivateImageID, privateLookups, true)
	runCases("", invalidLookups, false)
	runCases(testPrivateImageID, digestLookups, true)

	// now make local image fully qualified (`docker.io` will be prepended)
	store.Tag(testLocalImageName, "", testLocalImageID, false, false)
	store.Delete(testLocalImageName, "latest")

	if imageCount(store) != 3 {
		t.Fatalf("Expected three images in tag store, not %d.", imageCount(store))
	}
	corrupted := false
	for _, repoName := range []string{"docker.io/" + testLocalImageName, testPrivateImageName} {
		if repo, exists := store.Repositories[repoName]; !exists {
			corrupted = true
			break
		} else if _, exists := repo["latest"]; !exists {
			corrupted = true
			break
		}
	}
	if corrupted {
		logStoreContent(t, store, -1)
		t.Fatalf("TagStore got corrupted!")
	}

	// and retest lookups of local image - now prefixed with `docker.io`
	localLookups = []string{
		testLocalImageID,
		testLocalImageIDShort,
		testLocalImageName + ":" + testLocalImageID,
		testLocalImageName + ":" + testLocalImageIDShort,
		testLocalImageName,
		testLocalImageName + ":" + tags.DefaultTag,
		"docker.io/" + testLocalImageName,
		"docker.io/" + testLocalImageName + ":" + tags.DefaultTag,
		"index.docker.io/" + testLocalImageName,
		"index.docker.io/" + testLocalImageName + ":" + tags.DefaultTag,
		"library/" + testLocalImageName,
		"library/" + testLocalImageName + ":" + tags.DefaultTag,
		"docker.io/library/" + testLocalImageName,
		"docker.io/library/" + testLocalImageName + ":" + tags.DefaultTag,
		"index.docker.io/library/" + testLocalImageName,
		"index.docker.io/library/" + testLocalImageName + ":" + tags.DefaultTag,
	}

	invalidLookups = []string{
		testLocalImageName + ":" + "fail",
		"docker.io/" + testPrivateRemoteName,
		testPrivateIndexName + "/" + testLocalImageName,
		"fail:fail",
	}

	runCases(testLocalImageID, localLookups, true)
	runCases(testPrivateImageID, privateLookups, true)
	runCases("", invalidLookups, false)
	runCases(testPrivateImageID, digestLookups, true)
}

func TestValidateDigest(t *testing.T) {
	tests := []struct {
		input       string
		expectError bool
	}{
		{"", true},
		{"latest", true},
		{"sha256:b", false},
		{"tarsum+v1+sha256:bY852-_.+=", false},
		{"#$%#$^:$%^#$%", true},
	}

	for i, test := range tests {
		err := validateDigest(test.input)
		gotError := err != nil
		if e, a := test.expectError, gotError; e != a {
			t.Errorf("%d: with input %s, expected error=%t, got %t: %s", i, test.input, test.expectError, gotError, err)
		}
	}
}

type setRefCase struct {
	imageID         string
	dest            string
	destRef         string
	refIsDigest     bool
	keepUnqualified bool
	shallSucceed    bool
	expectedResult  string
}

var setRefCases = []setRefCase{
	{testLocalImageID, testLocalImageName, "", false, false, true, "docker.io/" + testLocalImageName},
	{testLocalImageID, testLocalImageName, "", false, true, false, ""},
	{testLocalImageID, testLocalImageName, "latest", false, false, true, "docker.io/" + testLocalImageName},
	{testLocalImageID, testLocalImageName, "latest", false, true, false, ""},
	{testLocalImageID, testLocalImageName, "foo", false, false, true, "docker.io/" + testLocalImageName},
	{testLocalImageID, testLocalImageName, "foo", false, true, true, testLocalImageName},
	{testLocalImageID, testLocalImageName, "bar", true, true, false, ""},
	{testLocalImageID, testLocalImageName, testPrivateImageDigest, true, true, true, testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "", false, false, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "", false, true, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "latest", false, false, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "latest", false, true, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "foo", false, false, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/" + testLocalImageName, "foo", false, true, true, "42.42.42.42:5001/" + testLocalImageName},
	{testLocalImageID, testPrivateImageName, "", false, false, false, ""},
	{testLocalImageID, testPrivateImageName, "", false, true, false, ""},
	{testLocalImageID, testPrivateImageName, "latest", false, false, false, ""},
	{testLocalImageID, testPrivateImageName, "latest", false, true, false, ""},
	{testLocalImageID, testPrivateImageName, "foo", false, false, true, testPrivateImageName},
	{testLocalImageID, testPrivateImageName, "foo", false, true, true, testPrivateImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "", false, false, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "", false, true, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "latest", false, false, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "latest", false, true, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "foo", false, false, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, testPrivateIndexName + "/" + testLocalImageName, "foo", false, true, true, testPrivateIndexName + "/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "", false, false, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "", false, true, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "latest", false, false, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "latest", false, true, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "foo", false, false, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testLocalImageID, "42.42.42.42:5001/library/" + testLocalImageName, "foo", false, true, true, "42.42.42.42:5001/library/" + testLocalImageName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "", false, true, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "latest", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "latest", false, true, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "foo", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/" + testPrivateRemoteName, "foo", false, true, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "", false, true, true, testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "latest", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "latest", false, true, true, testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "foo", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "foo", false, true, true, testPrivateRemoteName},
	{testPrivateImageID, testPrivateRemoteName, "bar", true, true, false, ""},
	{testPrivateImageID, testPrivateRemoteName, testPrivateImageDigest, true, true, true, testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "", false, true, true, "docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "latest", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "latest", false, true, true, "docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "foo", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "docker.io/library/" + testPrivateRemoteName, "foo", false, true, true, "docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "", false, true, true, "index.docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "latest", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "latest", false, true, true, "index.docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "foo", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/" + testPrivateRemoteName, "foo", false, true, true, "index.docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "", false, true, true, "index.docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "latest", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "latest", false, true, true, "index.docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "foo", false, false, true, "docker.io/" + testPrivateRemoteName},
	{testPrivateImageID, "index.docker.io/library/" + testPrivateRemoteName, "foo", false, true, true, "index.docker.io/library/" + testPrivateRemoteName},
	{testPrivateImageID, testLocalImageName, "", false, false, true, "docker.io/" + testLocalImageName},
	{testPrivateImageID, testLocalImageName, "", false, true, false, ""},
	{testPrivateImageID, testLocalImageName, "latest", false, false, true, "docker.io/" + testLocalImageName},
	{testPrivateImageID, testLocalImageName, "latest", false, true, false, ""},
	{testPrivateImageID, testLocalImageName, "foo", false, false, true, "docker.io/" + testLocalImageName},
	{testPrivateImageID, testLocalImageName, "foo", false, true, true, testLocalImageName},
}

func runSetTagCases(t *testing.T, store *TagStore, additionalRegistry string) {
	var err error

	localImages := map[string]string{
		testLocalImageID:   testLocalImageName,
		testPrivateImageID: testPrivateImageName,
	}

	for i, testCase := range setRefCases {
		for _, source := range []string{testCase.imageID, localImages[testCase.imageID]} {
			for _, sourceTag := range []string{"", "latest"} {
				if source == testCase.imageID && sourceTag != "" {
					continue
				}
				refSep := ":"
				if testCase.refIsDigest {
					refSep = "@"
				}
				taggedSource := source
				if sourceTag != "" {
					taggedSource = source + ":" + sourceTag
				}
				dest := testCase.dest
				expectedResult := testCase.expectedResult
				if !registry.RepositoryNameHasIndex(testCase.dest) && !testCase.keepUnqualified && additionalRegistry != "" {
					_, remoteName := registry.SplitReposName(expectedResult, false)
					expectedResult = additionalRegistry + "/" + remoteName
				}
				if testCase.destRef != "" {
					dest = testCase.dest + refSep + testCase.destRef
					expectedResult = expectedResult + refSep + testCase.destRef
				}

				if testCase.refIsDigest {
					err = store.SetDigest(testCase.dest, testCase.destRef, taggedSource, testCase.keepUnqualified)
				} else {
					err = store.Tag(testCase.dest, testCase.destRef, taggedSource, false, testCase.keepUnqualified)
				}

				if err == nil && !testCase.shallSucceed {
					logStoreContent(t, store, i)
					t.Fatalf("[case#%d] Tagging of %q as %q should have failed.", i, taggedSource, dest)
				}
				if err != nil && testCase.shallSucceed {
					logStoreContent(t, store, i)
					t.Errorf("[case#%d] Tagging of %q as %q should have succeeded: %v.", i, taggedSource, dest, err)
					continue
				}
				if err != nil {
					continue
				}

				if imageCount(store) != 4 {
					logStoreContent(t, store, i)
					t.Fatalf("[case#%d] Expected 4 images in TagStore, not %d.", i, imageCount(store))
				}

				if img, err := store.LookupImage(dest); err != nil {
					t.Errorf("[case#%d] Error looking up %q: %s", i, dest, err)
				} else if img == nil {
					t.Errorf("[case#%d] Expected 1 image, none found.", i)
				}

				if img, err := store.LookupImage(expectedResult); err != nil {
					t.Errorf("[case#%d] Error looking up %q: %s", i, expectedResult, err)
				} else if img == nil {
					t.Errorf("[case#%d] Expected 1 image, none found.", i)
				} else if img.ID != testCase.imageID {
					t.Errorf("[case#%d] Expected ID %q found %q", i, testCase.imageID, img.ID)
				}

				toDelete := expectedResult
				if strings.HasSuffix(expectedResult, refSep+testCase.destRef) {
					toDelete = expectedResult[:len(expectedResult)-len(refSep+testCase.destRef)]
				}
				if ok, err := store.Delete(toDelete, testCase.destRef); err != nil || !ok {
					logStoreContent(t, store, i)
					t.Fatalf("[case#%d] Deletion of %q should have succeeded: %v", i, expectedResult, err)
				}
				if imageCount(store) != 3 {
					logStoreContent(t, store, i)
					t.Fatalf("[case#%d] Expected 3 repositories in TagStore, not %d.", i, imageCount(store))
				}
				corrupted := false
				for _, repoName := range []string{testLocalImageName, testPrivateImageName} {
					if repo, exists := store.Repositories[repoName]; !exists {
						corrupted = true
						break
					} else if _, exists := repo["latest"]; !exists {
						corrupted = true
						break
					}
				}
				if !corrupted {
					if _, exists := store.Repositories[testPrivateImageName][testPrivateImageDigest]; !exists {
						corrupted = true
					}
				}
				if corrupted {
					logStoreContent(t, store, i)
					t.Fatalf("[case#%d] TagStore got corrupted after deletion of %q.", i, expectedResult)
				}
			}
		}
	}
}

func TestSetTag(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)
	defer store.graph.driver.Cleanup()

	runSetTagCases(t, store, "")
}

func TestSetTagWithAdditionalRegistry(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)
	defer store.graph.driver.Cleanup()

	registry.RegistryList = append([]string{"42.42.42.42:5001"}, registry.RegistryList...)
	defer func() {
		registry.RegistryList = registry.RegistryList[1:]
	}()

	runSetTagCases(t, store, "42.42.42.42:5001")
}
