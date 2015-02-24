package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// See issue docker/docker#8141
func TestPullImageWithAliases(t *testing.T) {
	defer setupRegistry(t)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURLs[0])
	defer deleteImages(repoName)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", repoName, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "tag", "busybox", repo)); err != nil {
			t.Fatalf("Failed to tag image %v: error %v, output %q", repos, err, out)
		}
		defer deleteImages(repo)
		if out, err := exec.Command(dockerBinary, "push", repo).CombinedOutput(); err != nil {
			t.Fatalf("Failed to push image %v: error %v, output %q", repo, err, string(out))
		}
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	if out, err := exec.Command(dockerBinary, args...).CombinedOutput(); err != nil {
		t.Fatalf("Failed to clean images: error %v, output %q", err, string(out))
	}

	// Pull a single tag and verify it doesn't bring down all aliases.
	pullCmd := exec.Command(dockerBinary, "pull", repos[0])
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		t.Fatalf("Failed to pull %v: error %v, output %q", repoName, err, out)
	}
	if err := exec.Command(dockerBinary, "inspect", repos[0]).Run(); err != nil {
		t.Fatalf("Image %v was not pulled down", repos[0])
	}
	for _, repo := range repos[1:] {
		if err := exec.Command(dockerBinary, "inspect", repo).Run(); err == nil {
			t.Fatalf("Image %v shouldn't have been pulled down", repo)
		}
	}

	logDone("pull - image with aliases")
}

// pulling library/hello-world should show verified message
func TestPullVerified(t *testing.T) {
	// Image must be pulled from central repository to get verified message
	// unless keychain is manually updated to contain the daemon's sign key.

	verifiedName := "hello-world"
	defer deleteImages(verifiedName)

	// pull it
	expected := "The image you are pulling has been verified"
	pullCmd := exec.Command(dockerBinary, "pull", verifiedName)
	if out, exitCode, err := runCommandWithOutput(pullCmd); err != nil || !strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			t.Skipf("pulling the '%s' image from the registry has failed: %s", verifiedName, err)
		}
		t.Fatalf("pulling a verified image failed. expected: %s\ngot: %s, %v", expected, out, err)
	}

	// pull it again
	pullCmd = exec.Command(dockerBinary, "pull", verifiedName)
	if out, exitCode, err := runCommandWithOutput(pullCmd); err != nil || strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			t.Skipf("pulling the '%s' image from the registry has failed: %s", verifiedName, err)
		}
		t.Fatalf("pulling a verified image failed. unexpected verify message\ngot: %s, %v", out, err)
	}

	logDone("pull - pull verified")
}

// pulling an image from the central registry should work
func TestPullImageFromCentralRegistry(t *testing.T) {
	defer deleteImages("hello-world")

	pullCmd := exec.Command(dockerBinary, "pull", "hello-world")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		t.Fatalf("pulling the hello-world image from the registry has failed: %s, %v", out, err)
	}
	logDone("pull - pull hello-world")
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func TestPullNonExistingImage(t *testing.T) {
	pullCmd := exec.Command(dockerBinary, "pull", "fooblahblah1234")
	if out, _, err := runCommandWithOutput(pullCmd); err == nil {
		t.Fatalf("expected non-zero exit status when pulling non-existing image: %s", out)
	}
	logDone("pull - pull fooblahblah1234 (non-existing image)")
}

// pulling an image from the central registry using official names should work
// ensure all pulls result in the same image
func TestPullImageOfficialNames(t *testing.T) {
	names := []string{
		"docker.io/hello-world",
		"index.docker.io/hello-world",
		"library/hello-world",
		"docker.io/library/hello-world",
		"index.docker.io/library/hello-world",
	}
	for _, name := range names {
		pullCmd := exec.Command(dockerBinary, "pull", name)
		out, exitCode, err := runCommandWithOutput(pullCmd)
		if err != nil || exitCode != 0 {
			t.Errorf("pulling the '%s' image from the registry has failed: %s", name, err)
			continue
		}

		// ensure we don't have multiple image names.
		imagesCmd := exec.Command(dockerBinary, "images")
		out, _, err = runCommandWithOutput(imagesCmd)
		if err != nil {
			t.Errorf("listing images failed with errors: %v", err)
		} else if strings.Contains(out, name) {
			if name != "docker.io/hello-world" {
				t.Errorf("images should not have listed '%s'", name)
			}
		}
	}
	logDone("pull - pull official names")
}

func TestPullFromAdditionalRegistry(t *testing.T) {
	reg := setupAndGetRegistryAt(t, privateRegistryURLs[0])
	defer reg.Close()
	d := NewDaemon(t)
	if err := d.StartWithBusybox("--add-registry=" + reg.url); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", reg.url, err)
	}
	defer d.Stop()

	busyboxId := d.getAndTestImageEntry(t, 1, reg.url+"/busybox", "").id

	// this will pull from docker.io
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"%s\": %v", reg.url, err)
	}

	helloWorldId := d.getAndTestImageEntry(t, 2, "docker.io/hello-world", "").id
	if helloWorldId == busyboxId {
		t.Fatalf("docker.io/hello-world must have different ID than busybox image")
	}

	// push busybox to additional registry as "library/hello-world" and remove all local images
	if out, err := d.Cmd("tag", reg.url+"/busybox", reg.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("push", reg.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg.url+"/library/hello-world", err, out)
	}
	toRemove := []string{reg.url + "/library/hello-world", reg.url + "/busybox", "docker.io/hello-world"}
	if out, err := d.Cmd("rmi", toRemove...); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// pull the same name again - now the image should be pulled from additional registry
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"%s\": %v", reg.url, err)
	}
	d.getAndTestImageEntry(t, 1, reg.url+"/library/hello-world", busyboxId)

	// empty images once more
	if out, err := d.Cmd("rmi", reg.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to remove image %s: %v, output: %s", reg.url+"library/hello-world", err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// now pull with fully qualified name
	if _, err := d.Cmd("pull", "docker.io/library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull docker.io/library/hello-world from \"%s\": %v", reg.url, err)
	}
	d.getAndTestImageEntry(t, 1, "docker.io/hello-world", helloWorldId)

	logDone("pull - image from additional registry")
}

func TestPullFromAdditionalRegistries(t *testing.T) {
	reg1 := setupAndGetRegistryAt(t, privateRegistryURLs[0])
	defer reg1.Close()
	reg2 := setupAndGetRegistryAt(t, privateRegistryURLs[1])
	defer reg2.Close()
	d := NewDaemon(t)
	daemonArgs := []string{"--add-registry=" + reg1.url, "--add-registry=" + reg2.url}
	if err := d.StartWithBusybox(daemonArgs...); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}
	defer d.Stop()

	busyboxId := d.getAndTestImageEntry(t, 1, reg1.url+"/busybox", "").id

	// this will pull from docker.io
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"docker.io\": %v", err)
	}
	helloWorldId := d.getAndTestImageEntry(t, 2, "docker.io/hello-world", "").id
	if helloWorldId == busyboxId {
		t.Fatalf("docker.io/hello-world must have different ID than busybox image")
	}

	// push:
	//  hello-world to 1st additional registry as "misc/hello-world"
	//  busybox to 2nd additional registry as "library/hello-world"
	if out, err := d.Cmd("tag", "docker.io/hello-world", reg1.url+"/misc/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", "docker.io/hello-world", err, out)
	}
	if out, err := d.Cmd("tag", reg1.url+"/busybox", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg1.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("push", reg1.url+"/misc/hello-world"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg1.url+"/misc/hello-world", err, out)
	}
	if out, err := d.Cmd("push", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg2.url+"/library/busybox", err, out)
	}
	// and remove all local images
	toRemove := []string{reg1.url + "/misc/hello-world", reg2.url + "/library/hello-world", reg1.url + "/busybox", "docker.io/hello-world"}
	if out, err := d.Cmd("rmi", toRemove...); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// now pull the "library/hello-world" from 2nd additional registry
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"%s\": %v", reg2.url, err)
	}
	d.getAndTestImageEntry(t, 1, reg2.url+"/library/hello-world", busyboxId)

	// now pull the "misc/hello-world" from 1st additional registry
	if _, err := d.Cmd("pull", "misc/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull misc/hello-world from \"%s\": %v", reg2.url, err)
	}
	d.getAndTestImageEntry(t, 2, reg1.url+"/misc/hello-world", helloWorldId)

	// tag it as library/hello-world and push it to 1st registry
	if out, err := d.Cmd("tag", reg1.url+"/misc/hello-world", reg1.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg1.url+"/misc/hello-world", err, out)
	}
	if out, err := d.Cmd("push", reg1.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg1.url+"/library/hello-world", err, out)
	}

	// remove all images
	toRemove = []string{reg1.url + "/misc/hello-world", reg1.url + "/library/hello-world", reg2.url + "/library/hello-world"}
	if out, err := d.Cmd("rmi", toRemove...); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// now pull "library/hello-world" from 1st additional registry
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"%s\": %v", reg1.url, err)
	}
	d.getAndTestImageEntry(t, 1, reg1.url+"/library/hello-world", helloWorldId)

	// now pull fully qualified image from 2nd registry
	if _, err := d.Cmd("pull", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull %s/library/hello-world: %v", reg2.url, err)
	}
	d.getAndTestImageEntry(t, 2, reg2.url+"/library/hello-world", busyboxId)

	logDone("pull - image from additional registries")
}

// Test pulls from blocked public registry and from private registry. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func doTestPullFromBlockedPublicRegistry(t *testing.T, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	reg := setupAndGetRegistryAt(t, privateRegistryURLs[0])
	defer reg.Close()
	d := NewDaemon(t)
	if err := d.StartWithBusybox(daemonArgs...); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}
	defer d.Stop()

	busyboxId := d.getAndTestImageEntry(t, 1, "busybox", "").id

	// try to pull from docker.io
	if out, err := d.Cmd("pull", "library/hello-world"); err == nil {
		t.Fatalf("pull from blocked public registry should have failed, output: %s", out)
	}

	// tag busybox as library/hello-world and push it to some private registry
	if out, err := d.Cmd("tag", "busybox", reg.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := d.Cmd("push", reg.url+"/library/hello-world"); !allBlocked && err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg.url+"/library/hello-world", err, out)
	} else if allBlocked && err == nil {
		t.Fatalf("push to private registry should have failed, output: %q", out)
	}

	// remove library/hello-world image
	if out, err := d.Cmd("rmi", reg.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", reg.url+"/library/hello-world", err, out)
	}
	d.getAndTestImageEntry(t, 1, "busybox", busyboxId)

	// try to pull from private registry
	if out, err := d.Cmd("pull", reg.url+"/library/hello-world"); !allBlocked && err != nil {
		t.Fatalf("we should have been able to pull %s/library/hello-world: %v", reg.url, err)
	} else if allBlocked && err == nil {
		t.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		d.getAndTestImageEntry(t, 2, reg.url+"/library/hello-world", busyboxId)
	}
}

func TestPullFromBlockedPublicRegistry(t *testing.T) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		doTestPullFromBlockedPublicRegistry(t, []string{"--block-registry=" + blockedRegistry})
	}
	logDone("pull - image from blocked public registry")
}

func TestPullWithAllRegistriesBlocked(t *testing.T) {
	doTestPullFromBlockedPublicRegistry(t, []string{"--block-registry=all"})
	logDone("pull - image with all registries blocked")
}

// Test pulls from additional registry with public registry blocked. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func doTestPullFromPrivateRegistriesWithPublicBlocked(t *testing.T, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	// additional registry
	reg1 := setupAndGetRegistryAt(t, privateRegistryURLs[0])
	defer reg1.Close()
	// private registry
	reg2 := setupAndGetRegistryAt(t, privateRegistryURLs[1])
	defer reg2.Close()
	d := NewDaemon(t)
	daemonArgs = append(daemonArgs, "--add-registry="+reg1.url)
	if err := d.StartWithBusybox(daemonArgs...); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}
	defer d.Stop()

	busyboxId := d.getAndTestImageEntry(t, 1, reg1.url+"/busybox", "").id

	// try to pull from blocked public registry
	if out, err := d.Cmd("pull", "library/hello-world"); err == nil {
		t.Fatalf("pulling from blocked public registry should have failed, output: %s", out)
	}

	// push busybox to
	//  additional registry as "misc/busybox"
	//  private registry as "library/busybox"
	// and remove all local images
	if out, err := d.Cmd("tag", reg1.url+"/busybox", reg1.url+"/misc/busybox"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg1.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("tag", reg1.url+"/busybox", reg2.url+"/library/busybox"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg1.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("push", reg1.url+"/misc/busybox"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg1.url+"/misc/busybox", err, out)
	}
	if out, err := d.Cmd("push", reg2.url+"/library/busybox"); !allBlocked && err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg2.url+"/library/busybox", err, out)
	} else if allBlocked && err == nil {
		t.Fatalf("push to private registry should have failed, output: %q", out)
	}
	toRemove := []string{reg1.url + "/busybox", reg1.url + "/misc/busybox", reg2.url + "/library/busybox"}
	if out, err := d.Cmd("rmi", toRemove...); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// try to pull "library/busybox" from additional registry
	if out, err := d.Cmd("pull", "library/busybox"); err == nil {
		t.Fatalf("pull of library/busybox from additional registry should have failed, output: %q", out)
	}

	// now pull the "misc/busybox" from additional registry
	if _, err := d.Cmd("pull", "misc/busybox"); err != nil {
		t.Fatalf("we should have been able to pull misc/hello-world from \"%s\": %v", reg1.url, err)
	}
	d.getAndTestImageEntry(t, 1, reg1.url+"/misc/busybox", busyboxId)

	// try to pull "library/busybox" from private registry
	if out, err := d.Cmd("pull", reg2.url+"/library/busybox"); !allBlocked && err != nil {
		t.Fatalf("we should have been able to pull %s/library/busybox: %v", reg2.url, err)
	} else if allBlocked && err == nil {
		t.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		d.getAndTestImageEntry(t, 2, reg2.url+"/library/busybox", busyboxId)
	}
}

func TestPullFromPrivateRegistriesWithPublicBlocked(t *testing.T) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		doTestPullFromPrivateRegistriesWithPublicBlocked(t, []string{"--block-registry=" + blockedRegistry})
	}
	logDone("pull - image from private registries with public registry blocked")
}

func TestPullFromAdditionalRegistryWithAllBlocked(t *testing.T) {
	doTestPullFromPrivateRegistriesWithPublicBlocked(t, []string{"--block-registry=all"})
	logDone("pull - image from additional registry with all the others blocked")
}

func TestPullFromBlockedRegistry(t *testing.T) {
	// blocked registry
	reg1 := setupAndGetRegistryAt(t, privateRegistryURLs[0])
	defer reg1.Close()
	// additional registry
	reg2 := setupAndGetRegistryAt(t, privateRegistryURLs[1])
	defer reg2.Close()
	d := NewDaemon(t)
	daemonArgs := []string{"--block-registry=" + reg1.url, "--add-registry=" + reg2.url}
	if err := d.StartWithBusybox(daemonArgs...); err != nil {
		t.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}
	defer d.Stop()

	busyboxId := d.getAndTestImageEntry(t, 1, reg2.url+"/busybox", "").id

	// pull image from docker.io
	if _, err := d.Cmd("pull", "library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"docker.io\": %v", err)
	}
	helloWorldId := d.getAndTestImageEntry(t, 2, "docker.io/hello-world", "").id
	if helloWorldId == busyboxId {
		t.Fatalf("docker.io/hello-world must have different ID than busybox image")
	}

	// push "hello-world to blocked and additional registry and remove all local images
	if out, err := d.Cmd("tag", reg2.url+"/busybox", reg1.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg2.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("tag", reg2.url+"/busybox", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to tag image %s: error %v, output %q", reg2.url+"/busybox", err, out)
	}
	if out, err := d.Cmd("push", reg1.url+"/library/hello-world"); err == nil {
		t.Fatalf("push to blocked registry should have failed, output: %q", out)
	}
	if out, err := d.Cmd("push", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("failed to push image %s: error %v, output %q", reg2.url+"/library/hello-world", err, out)
	}
	toRemove := []string{reg1.url + "/library/hello-world", reg2.url + "/library/hello-world", "docker.io/hello-world", reg2.url + "/busybox"}
	if out, err := d.Cmd("rmi", toRemove...); err != nil {
		t.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	d.getAndTestImageEntry(t, 0, "", "")

	// try to pull "library/hello-world" from blocked registry
	if out, err := d.Cmd("pull", reg1.url+"/library/hello-world"); err == nil {
		t.Fatalf("pull of library/hello-world from additional registry should have failed, output: %q", out)
	}

	// now pull the "library/hello-world" from additional registry
	if _, err := d.Cmd("pull", reg2.url+"/library/hello-world"); err != nil {
		t.Fatalf("we should have been able to pull library/hello-world from \"%s\": %v", reg2.url, err)
	}
	d.getAndTestImageEntry(t, 1, reg2.url+"/library/hello-world", busyboxId)

	logDone("pull - image from blocked registry")
}
