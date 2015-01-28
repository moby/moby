package main

/*

This validator tool is intended to be run within an App Container Executor
(ACE), and verifies that the ACE has been set up correctly.

This verifies the _apps perspective_ of the execution environment.

Changes to the validator need to be reflected in app_manifest.json, and vice-versa

The App Container Execution spec defines the following expectations within the execution environment:
 - Working Directory defaults to the root of the application image, overridden with "workingDirectory"
 - PATH /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
 - USER, LOGNAME username of the user executing this app
 - HOME home directory of the user
 - SHELL login shell of the user
 - AC_APP_NAME the entrypoint that this process was defined from

In addition, we validate:
 - The expected mount points are mounted
 - metadata service reachable at http://169.254.169.255

TODO(jonboulle):
 - should we validate Isolators? (e.g. MemoryLimit + malloc, or capabilities)
 - should we validate ports? (e.g. that they are available to bind to within the network namespace of the container)

*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

const (
	standardPath     = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	appNameEnv       = "AC_APP_NAME"
	metadataPathBase = "/acMetadata/v1"

	// marker files to validate
	prestartFile = "/prestart"
	mainFile     = "/main"
	poststopFile = "/poststop"

	mainVolFile     = "/db/main"
	sidekickVolFile = "/db/sidekick"

	timeout = 5 * time.Second
)

var (
	// Expected values must be kept in sync with app_manifest.json
	workingDirectory = "/opt/acvalidator"
	// "Environment"
	env = map[string]string{
		"IN_ACE_VALIDATOR": "correct",
		"HOME":             "/root",
		"USER":             "root",
		"LOGNAME":          "root",
		"SHELL":            "/bin/sh",
	}
	// "MountPoints"
	mps = map[string]types.MountPoint{
		"database": types.MountPoint{
			Path:     "/db",
			ReadOnly: false,
		},
	}
	// "Name"
	an = "coreos.com/ace-validator-main"
)

type results []error

// main outputs diagnostic information to stderr and exits 1 if validation fails
func main() {
	if len(os.Args) != 2 {
		stderr("usage: %s [main|sidekick|preStart|postStop]", os.Args[0])
		os.Exit(64)
	}
	mode := os.Args[1]
	var res results
	switch strings.ToLower(mode) {
	case "main":
		res = validateMain()
	case "sidekick":
		res = validateSidekick()
	case "prestart":
		res = validatePrestart()
	case "poststop":
		res = validatePoststop()
	default:
		stderr("unrecognized mode: %s", mode)
		os.Exit(64)
	}
	if len(res) == 0 {
		fmt.Printf("%s OK\n", mode)
		os.Exit(0)
	}
	fmt.Printf("%s FAIL\n", mode)
	for _, err := range res {
		fmt.Fprintln(os.Stderr, "==>", err)
	}
	os.Exit(1)
}

func validateMain() (errs results) {
	errs = append(errs, assertExists(prestartFile)...)
	errs = append(errs, assertNotExistsAndCreate(mainFile)...)
	errs = append(errs, assertNotExists(poststopFile)...)
	errs = append(errs, ValidatePath(standardPath)...)
	errs = append(errs, ValidateWorkingDirectory(workingDirectory)...)
	errs = append(errs, ValidateEnvironment(env)...)
	errs = append(errs, ValidateMountpoints(mps)...)
	errs = append(errs, ValidateAppNameEnv(an)...)
	errs = append(errs, ValidateMetadataSvc()...)
	errs = append(errs, waitForFile(sidekickVolFile, timeout)...)
	errs = append(errs, assertNotExistsAndCreate(mainVolFile)...)
	return
}

func validateSidekick() (errs results) {
	errs = append(errs, assertNotExistsAndCreate(sidekickVolFile)...)
	errs = append(errs, waitForFile(mainVolFile, timeout)...)
	return
}

func validatePrestart() (errs results) {
	errs = append(errs, assertNotExistsAndCreate(prestartFile)...)
	errs = append(errs, assertNotExists(mainFile)...)
	errs = append(errs, assertNotExists(poststopFile)...)
	return
}

func validatePoststop() (errs results) {
	errs = append(errs, assertExists(prestartFile)...)
	errs = append(errs, assertExists(mainFile)...)
	errs = append(errs, assertNotExistsAndCreate(poststopFile)...)
	return
}

// ValidatePath ensures that the PATH has been set up correctly within the
// environment in which this process is being run
func ValidatePath(wp string) results {
	r := results{}
	gp := os.Getenv("PATH")
	if wp != gp {
		r = append(r, fmt.Errorf("PATH not set appropriately (need %q)", wp))
	}
	return r
}

// ValidateWorkingDirectory ensures that the process working directory is set
// to the desired path.
func ValidateWorkingDirectory(wwd string) (r results) {
	wd, err := os.Getwd()
	if err != nil {
		r = append(r, fmt.Errorf("error getting working directory: %x", err))
		return
	}
	if wd != wwd {
		r = append(r, fmt.Errorf("working directory %q not set (need %q)", wwd, wd))
	}
	return
}

// ValidateEnvironment ensures that the given environment exactly maps the
// environment in which this process is running
func ValidateEnvironment(wenv map[string]string) (r results) {
	for wkey, wval := range wenv {
		gval := os.Getenv(wkey)
		if gval != wval {
			err := fmt.Errorf("environment variable %q not set as expected (need %q)", wkey, wval)
			r = append(r, err)
		}
	}
	for _, s := range os.Environ() {
		parts := strings.SplitN(s, "=", 2)
		k := parts[0]
		_, ok := wenv[k]
		switch {
		case k == appNameEnv, k == "PATH", k == "TERM", k == "AC_METADATA_URL":
		case !ok:
			r = append(r, fmt.Errorf("unexpected environment variable %q set", k))
		}
	}
	return
}

// ValidateAppNameEnv ensures that the environment variable specifying the
// entrypoint of this process is set correctly.
func ValidateAppNameEnv(want string) (r results) {
	if got := os.Getenv(appNameEnv); got != want {
		r = append(r, fmt.Errorf("%s not set appropriately", appNameEnv))
	}
	return
}

// ValidateMountpoints ensures that the given mount points are present in the
// environment in which this process is running
func ValidateMountpoints(wmp map[string]types.MountPoint) results {
	r := results{}
	// TODO(jonboulle): verify actual source
	for _, mp := range wmp {
		if err := checkMount(mp.Path, mp.ReadOnly); err != nil {
			r = append(r, err)
		}
	}
	return r
}

func metadataRequest(req *http.Request) ([]byte, error) {
	cli := http.Client{
		Timeout: 100 * time.Millisecond,
	}

	req.Header["Metadata-Flavor"] = []string{"AppContainer"}

	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Get %s failed with %v", req.URL, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Get %s failed on body read: %v", req.URL, err)
	}

	return body, nil
}

func metadataGet(metadataURL, path string) ([]byte, error) {
	uri := metadataURL + metadataPathBase + path
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		panic(err)
	}
	return metadataRequest(req)
}

func metadataPost(metadataURL, path string, body []byte) ([]byte, error) {
	uri := metadataURL + metadataPathBase + path
	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "text/plan")

	return metadataRequest(req)
}

func metadataPostForm(metadataURL, path string, data url.Values) ([]byte, error) {
	uri := metadataURL + metadataPathBase + path
	req, err := http.NewRequest("POST", uri, strings.NewReader(data.Encode()))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return metadataRequest(req)
}

func validateContainerAnnotations(metadataURL string, crm *schema.ContainerRuntimeManifest) results {
	r := results{}

	var actualAnnots types.Annotations

	annots, err := metadataGet(metadataURL, "/container/annotations/")
	if err != nil {
		return append(r, err)
	}

	for _, key := range strings.Split(string(annots), "\n") {
		if key == "" {
			continue
		}

		val, err := metadataGet(metadataURL, "/container/annotations/"+key)
		if err != nil {
			r = append(r, err)
		}

		name, err := types.NewACName(key)
		if err != nil {
			r = append(r, fmt.Errorf("invalid annotation label: %v", err))
			continue
		}

		actualAnnots.Set(*name, string(val))
	}

	if !reflect.DeepEqual(actualAnnots, crm.Annotations) {
		r = append(r, fmt.Errorf("container annotations mismatch: %v vs %v", actualAnnots, crm.Annotations))
	}

	return r
}

func validateContainerMetadata(metadataURL string, crm *schema.ContainerRuntimeManifest) results {
	r := results{}

	uid, err := metadataGet(metadataURL, "/container/uid")
	if err != nil {
		return append(r, err)
	}
	if strings.ToLower(string(uid)) != strings.ToLower(crm.UUID.String()) {
		return append(r, fmt.Errorf("UUID mismatch: %v vs %v", string(uid), crm.UUID.String()))
	}

	return append(r, validateContainerAnnotations(metadataURL, crm)...)
}

func validateAppAnnotations(metadataURL string, crm *schema.ContainerRuntimeManifest, app *schema.ImageManifest) results {
	r := results{}

	// build a map of expected annotations by merging app.Annotations
	// with ContainerRuntimeManifest overrides
	expectedAnnots := app.Annotations
	a := crm.Apps.Get(app.Name)
	if a == nil {
		panic("could not find app in manifest!")
	}
	for _, annot := range a.Annotations {
		expectedAnnots.Set(annot.Name, annot.Value)
	}

	var actualAnnots types.Annotations

	annots, err := metadataGet(metadataURL, "/apps/"+string(app.Name)+"/annotations/")
	if err != nil {
		return append(r, err)
	}

	for _, key := range strings.Split(string(annots), "\n") {
		if key == "" {
			continue
		}

		val, err := metadataGet(metadataURL, "/apps/"+string(app.Name)+"/annotations/"+key)
		if err != nil {
			r = append(r, err)
		}

		lbl, err := types.NewACName(key)
		if err != nil {
			r = append(r, fmt.Errorf("invalid annotation label: %v", err))
			continue
		}

		actualAnnots.Set(*lbl, string(val))
	}

	if !reflect.DeepEqual(actualAnnots, expectedAnnots) {
		err := fmt.Errorf("%v annotations mismatch: %v vs %v", app.Name, actualAnnots, expectedAnnots)
		r = append(r, err)
	}

	return r
}

func validateAppMetadata(metadataURL string, crm *schema.ContainerRuntimeManifest, a schema.RuntimeApp) results {
	appName := a.Name
	r := results{}

	am, err := metadataGet(metadataURL, "/apps/"+string(appName)+"/image/manifest")
	if err != nil {
		return append(r, err)
	}

	app := &schema.ImageManifest{}
	if err = json.Unmarshal(am, app); err != nil {
		return append(r, fmt.Errorf("failed to JSON-decode %q manifest: %v", string(appName), err))
	}

	id, err := metadataGet(metadataURL, "/apps/"+string(appName)+"/image/id")
	if err != nil {
		r = append(r, err)
	}

	if string(id) != a.ImageID.String() {
		err = fmt.Errorf("%q's image id mismatch: %v vs %v", string(appName), id, a.ImageID)
		r = append(r, err)
	}

	return append(r, validateAppAnnotations(metadataURL, crm, app)...)
}

func validateSigning(metadataURL string, crm *schema.ContainerRuntimeManifest) results {
	r := results{}

	plaintext := "Old MacDonald Had A Farm"

	// Sign
	sig, err := metadataPostForm(metadataURL, "/container/hmac/sign", url.Values{
		"content": []string{plaintext},
	})
	if err != nil {
		return append(r, err)
	}

	// Verify
	_, err = metadataPostForm(metadataURL, "/container/hmac/verify", url.Values{
		"content":   []string{plaintext},
		"uid":       []string{crm.UUID.String()},
		"signature": []string{string(sig)},
	})

	if err != nil {
		return append(r, err)
	}

	return r
}

func ValidateMetadataSvc() results {
	r := results{}

	metadataURL := os.Getenv("AC_METADATA_URL")
	if metadataURL == "" {
		return append(r, fmt.Errorf("AC_METADATA_URL is not set"))
	}

	cm, err := metadataGet(metadataURL, "/container/manifest")
	if err != nil {
		return append(r, err)
	}

	crm := &schema.ContainerRuntimeManifest{}
	if err = json.Unmarshal(cm, crm); err != nil {
		return append(r, fmt.Errorf("failed to JSON-decode container manifest: %v", err))
	}

	r = append(r, validateContainerMetadata(metadataURL, crm)...)

	for _, app := range crm.Apps {
		app := app
		r = append(r, validateAppMetadata(metadataURL, crm, app)...)
	}

	return append(r, validateSigning(metadataURL, crm)...)
}

// checkMount checks that the given string is a mount point, and that it is
// mounted appropriately read-only or not according to the given bool
func checkMount(d string, readonly bool) error {
	// or....
	// os.Stat(path).Sys().(*syscall.Stat_t).Dev
	sfs1 := &syscall.Statfs_t{}
	if err := syscall.Statfs(d, sfs1); err != nil {
		return fmt.Errorf("error calling statfs on %q: %v", d, err)
	}
	sfs2 := &syscall.Statfs_t{}
	if err := syscall.Statfs(filepath.Dir(d), sfs2); err != nil {
		return fmt.Errorf("error calling statfs on %q: %v", d, err)
	}
	if sfs1.Fsid == sfs2.Fsid {
		return fmt.Errorf("%q is not a mount point", d)
	}
	ro := sfs1.Flags&syscall.O_RDONLY == 1
	if ro != readonly {
		return fmt.Errorf("%q mounted ro=%t, want %t", d, ro, readonly)
	}
	return nil
}

// assertNotExistsAndCreate asserts that a file at the given path does not
// exist, and then proceeds to create (touch) the file. It returns any errors
// encountered at either of these steps.
func assertNotExistsAndCreate(p string) []error {
	var errs []error
	errs = append(errs, assertNotExists(p)...)
	if err := touchFile(p); err != nil {
		errs = append(errs, fmt.Errorf("error touching file %q: %v", p, err))
	}
	return errs
}

// assertNotExists asserts that a file at the given path does not exist. A
// non-empty list of errors is returned if the file exists or any error is
// encountered while checking.
func assertNotExists(p string) []error {
	var errs []error
	e, err := fileExists(p)
	if err != nil {
		errs = append(errs, fmt.Errorf("error checking %q exists: %v", p, err))
	}
	if e {
		errs = append(errs, fmt.Errorf("file %q exists unexpectedly", p))
	}
	return errs
}

// assertExists asserts that a file exists at the given path. A non-empty
// list of errors is returned if the file does not exist or any error is
// encountered while checking.
func assertExists(p string) []error {
	var errs []error
	e, err := fileExists(p)
	if err != nil {
		errs = append(errs, fmt.Errorf("error checking %q exists: %v", p, err))
	}
	if !e {
		errs = append(errs, fmt.Errorf("file %q does not exist as expected", p))
	}
	return errs
}

// touchFile creates an empty file, returning any error encountered
func touchFile(p string) error {
	_, err := os.Create(p)
	return err
}

// fileExists checks whether a file exists at the given path
func fileExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// waitForFile waits for the file at the given path to appear
func waitForFile(p string, to time.Duration) []error {
	done := time.After(to)
	for {
		select {
		case <-done:
			return []error{
				fmt.Errorf("timed out waiting for %s", p),
			}
		case <-time.After(1):
			if ok, _ := fileExists(p); ok {
				return nil
			}
		}
	}
}

func stderr(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
}
