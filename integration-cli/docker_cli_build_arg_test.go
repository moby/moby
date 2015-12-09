package main

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildBuildTimeArg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN echo $%s
		CMD echo $%s`, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); out != "\n" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgHistory(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envDef := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=%s`, envKey, envDef)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	out, _ := dockerCmd(c, "history", "--no-trunc", imgName)
	outputTabs := strings.Split(out, "\n")[1]
	if !strings.Contains(outputTabs, envDef) {
		c.Fatalf("failed to find arg default in image history output: %q expected: %q", outputTabs, envDef)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgCacheHit(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN echo $%s`, envKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachehit"
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID != origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build didn't use cache! expected image id: %q built image id: %q", origImgID, newImgID)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgCacheMissExtraArg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	extraEnvKey := "foo1"
	extraEnvVal := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}

	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ARG %s
		RUN echo $%s`, envKey, extraEnvKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachemiss"
	args = append(args, "--build-arg", fmt.Sprintf("%s=%s", extraEnvKey, extraEnvVal))
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID == origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build used cache, expected a miss!")
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgCacheMissSameArgDiffVal(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	newEnvVal := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}

	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN echo $%s`, envKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachemiss"
	args = []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, newEnvVal),
	}
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID == origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build used cache, expected a miss!")
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgOverrideArgDefinedBeforeEnv(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ENV %s %s
		RUN echo $%s
		CMD echo $%s
        `, envKey, envKey, envValOveride, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgOverrideEnvDefinedBeforeArg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ENV %s %s
		ARG %s
		RUN echo $%s
		CMD echo $%s
        `, envKey, envValOveride, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgExpansion(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldvarstest"

	wdVar := "WDIR"
	wdVal := "/tmp/"
	addVar := "AFILE"
	addVal := "addFile"
	copyVar := "CFILE"
	copyVal := "copyFile"
	envVar := "foo"
	envVal := "bar"
	exposeVar := "EPORT"
	exposeVal := "9999"
	userVar := "USER"
	userVal := "testUser"
	volVar := "VOL"
	volVal := "/testVol/"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", wdVar, wdVal),
		"--build-arg", fmt.Sprintf("%s=%s", addVar, addVal),
		"--build-arg", fmt.Sprintf("%s=%s", copyVar, copyVal),
		"--build-arg", fmt.Sprintf("%s=%s", envVar, envVal),
		"--build-arg", fmt.Sprintf("%s=%s", exposeVar, exposeVal),
		"--build-arg", fmt.Sprintf("%s=%s", userVar, userVal),
		"--build-arg", fmt.Sprintf("%s=%s", volVar, volVal),
	}
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
		ARG %s
		WORKDIR ${%s}
		ARG %s
		ADD ${%s} testDir/
		ARG %s
		COPY $%s testDir/
		ARG %s
		ENV %s=${%s}
		ARG %s
		EXPOSE $%s
		ARG %s
		USER $%s
		ARG %s
		VOLUME ${%s}`,
		wdVar, wdVar, addVar, addVar, copyVar, copyVar, envVar, envVar,
		envVar, exposeVar, exposeVar, userVar, userVar, volVar, volVar),
		map[string]string{
			addVal:  "some stuff",
			copyVal: "some stuff",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(imgName, ctx, true, args...); err != nil {
		c.Fatal(err)
	}

	var resMap map[string]interface{}
	var resArr []string
	res := ""
	res, err = inspectField(imgName, "Config.WorkingDir")
	if err != nil {
		c.Fatal(err)
	}
	if res != wdVal {
		c.Fatalf("Config.WorkingDir value mismatch. Expected: %s, got: %s", wdVal, res)
	}

	err = inspectFieldAndMarshall(imgName, "Config.Env", &resArr)
	if err != nil {
		c.Fatal(err)
	}

	found := false
	for _, v := range resArr {
		if fmt.Sprintf("%s=%s", envVar, envVal) == v {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("Config.Env value mismatch. Expected <key=value> to exist: %s=%s, got: %v",
			envVar, envVal, resArr)
	}

	err = inspectFieldAndMarshall(imgName, "Config.ExposedPorts", &resMap)
	if err != nil {
		c.Fatal(err)
	}
	if _, ok := resMap[fmt.Sprintf("%s/tcp", exposeVal)]; !ok {
		c.Fatalf("Config.ExposedPorts value mismatch. Expected exposed port: %s/tcp, got: %v", exposeVal, resMap)
	}

	res, err = inspectField(imgName, "Config.User")
	if err != nil {
		c.Fatal(err)
	}
	if res != userVal {
		c.Fatalf("Config.User value mismatch. Expected: %s, got: %s", userVal, res)
	}

	err = inspectFieldAndMarshall(imgName, "Config.Volumes", &resMap)
	if err != nil {
		c.Fatal(err)
	}
	if _, ok := resMap[volVal]; !ok {
		c.Fatalf("Config.Volumes value mismatch. Expected volume: %s, got: %v", volVal, resMap)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgExpansionOverride(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldvarstest"
	envKey := "foo"
	envVal := "bar"
	envKey1 := "foo1"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ENV %s %s
		ENV %s ${%s}
		RUN echo $%s
		CMD echo $%s`, envKey, envKey, envValOveride, envKey1, envKey, envKey1, envKey1)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgUntrustedDefinedAfterUse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		ARG %s
		CMD echo $%s`, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("able to access environment variable in output: %q expected to be missing", out)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); out != "\n" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgBuiltinArg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "HTTP_PROXY"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		CMD echo $%s`, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); out != "\n" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgDefaultOverride(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envValOveride),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=%s
		ENV %s $%s
		RUN echo $%s
		CMD echo $%s`, envKey, envVal, envKey, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 1 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgMultiArgsSameLine(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envKey1 := "foo1"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s %s`, envKey, envKey1)

	errStr := "ARG requires exactly one argument definition"
	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err == nil {
		c.Fatalf("build succeeded, expected to fail. Output: %v", out)
	} else if !strings.Contains(out, errStr) {
		c.Fatalf("Unexpected error. output: %q, expected error: %q", out, errStr)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgUnconsumedArg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		CMD echo $%s`, envKey, envKey)

	errStr := "One or more build-args"
	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err == nil {
		c.Fatalf("build succeeded, expected to fail. Output: %v", out)
	} else if !strings.Contains(out, errStr) {
		c.Fatalf("Unexpected error. output: %q, expected error: %q", out, errStr)
	}

}
func (s *DockerSuite) TestBuildBuildTimeArgQuotedValVariants(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envKey1 := "foo1"
	envKey2 := "foo2"
	envKey3 := "foo3"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=""
		ARG %s=''
		ARG %s="''"
		ARG %s='""'
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]`, envKey, envKey1, envKey2, envKey3,
		envKey, envKey2, envKey, envKey3, envKey1, envKey2, envKey1, envKey3,
		envKey2, envKey3)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil {
		c.Fatalf("build failed to complete: %q %q", out, err)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgEmptyValVariants(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	envKey1 := "foo1"
	envKey2 := "foo2"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=
		ARG %s=""
		ARG %s=''
		RUN [ "$%s" == "$%s" ]
		RUN [ "$%s" == "$%s" ]
		RUN [ "$%s" == "$%s" ]`, envKey, envKey1, envKey2, envKey, envKey1, envKey1, envKey2, envKey, envKey2)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil {
		c.Fatalf("build failed to complete: %q %q", out, err)
	}
}
func (s *DockerSuite) TestBuildBuildTimeArgDefintionWithNoEnvInjection(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imgName := "bldargtest"
	envKey := "foo"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN env`, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envKey) != 1 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("unexpected number of occurrences of the arg in output: %q expected: 1", out)
	}
}
