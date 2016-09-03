// +build linux freebsd

package graphtest

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

// DriverBenchExists benchmarks calls to exist
func DriverBenchExists(c *check.C, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		if !driver.Exists(base) {
			c.Fatal("Newly created image doesn't exist")
		}
	}
}

// DriverBenchGetEmpty benchmarks calls to get on an empty layer
func DriverBenchGetEmpty(c *check.C, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		_, err := driver.Get(base, "")
		c.StopTimer()
		if err != nil {
			c.Fatalf("Error getting mount: %s", err)
		}
		if err := driver.Put(base); err != nil {
			c.Fatalf("Error putting mount: %s", err)
		}
		c.StartTimer()
	}
}

// DriverBenchDiffBase benchmarks calls to diff on a root layer
func DriverBenchDiffBase(c *check.C, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addFiles(driver, base, 3); err != nil {
		c.Fatal(err)
	}

	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		arch, err := driver.Diff(base, "")
		if err != nil {
			c.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, arch)
		if err != nil {
			c.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDiffN benchmarks calls to diff on two layers with
// a provided number of files on the lower and upper layers.
func DriverBenchDiffN(c *check.C, bottom, top int, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, base, bottom, 3); err != nil {
		c.Fatal(err)
	}

	if err := driver.Create(upper, base, "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, upper, top, 6); err != nil {
		c.Fatal(err)
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		arch, err := driver.Diff(upper, "")
		if err != nil {
			c.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, arch)
		if err != nil {
			c.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDiffApplyN benchmarks calls to diff and apply together
func DriverBenchDiffApplyN(c *check.C, fileCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, base, fileCount, 3); err != nil {
		c.Fatal(err)
	}

	if err := driver.Create(upper, base, "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, upper, fileCount, 6); err != nil {
		c.Fatal(err)
	}
	diffSize, err := driver.DiffSize(upper, "")
	if err != nil {
		c.Fatal(err)
	}
	c.ResetTimer()
	c.StopTimer()
	for i := 0; i < c.N; i++ {
		diff := stringid.GenerateRandomID()
		if err := driver.Create(diff, base, "", nil); err != nil {
			c.Fatal(err)
		}

		if err := checkManyFiles(driver, diff, fileCount, 3); err != nil {
			c.Fatal(err)
		}

		c.StartTimer()

		arch, err := driver.Diff(upper, "")
		if err != nil {
			c.Fatal(err)
		}

		applyDiffSize, err := driver.ApplyDiff(diff, "", arch)
		if err != nil {
			c.Fatal(err)
		}

		c.StopTimer()
		arch.Close()

		if applyDiffSize != diffSize {
			// TODO: enforce this
			//c.Fatalf("Apply diff size different, got %d, expected %s", applyDiffSize, diffSize)
		}
		if err := checkManyFiles(driver, diff, fileCount, 6); err != nil {
			c.Fatal(err)
		}
	}
}

// DriverBenchDeepLayerDiff benchmarks calls to diff on top of a given number of layers.
func DriverBenchDeepLayerDiff(c *check.C, layerCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addFiles(driver, base, 50); err != nil {
		c.Fatal(err)
	}

	topLayer, err := addManyLayers(driver, base, layerCount)
	if err != nil {
		c.Fatal(err)
	}

	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		arch, err := driver.Diff(topLayer, "")
		if err != nil {
			c.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, arch)
		if err != nil {
			c.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDeepLayerRead benchmarks calls to read a file under a given number of layers.
func DriverBenchDeepLayerRead(c *check.C, layerCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(c, drivername, driveroptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	content := []byte("test content")
	if err := addFile(driver, base, "testfile.txt", content); err != nil {
		c.Fatal(err)
	}

	topLayer, err := addManyLayers(driver, base, layerCount)
	if err != nil {
		c.Fatal(err)
	}

	root, err := driver.Get(topLayer, "")
	if err != nil {
		c.Fatal(err)
	}
	defer driver.Put(topLayer)

	c.ResetTimer()
	for i := 0; i < c.N; i++ {

		// Read content
		co, err := ioutil.ReadFile(filepath.Join(root, "testfile.txt"))
		if err != nil {
			c.Fatal(err)
		}

		c.StopTimer()
		if bytes.Compare(co, content) != 0 {
			c.Fatalf("Wrong content in file %v, expected %v", co, content)
		}
		c.StartTimer()
	}
}
