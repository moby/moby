/*
package example illustrates results of "//go:fix replace"

It currently tests for combinations of:

	// Deprecated: use []string instead.
	//
	//go:fix inline
	type StrSlice = []string

	// Deprecated: use [New] instead.
	//
	//go:fix inline
	func NewClientWithOpts(ops ...Opt) (*Client, error) {
		return New(ops...)
	}

	// Deprecated: use [WithAPIVersion] instead.
	//
	//go:fix inline
	func WithVersion(version string) Opt {
		return WithAPIVersion(version)
	}

	// Deprecated: use [WithAPIVersionFromEnv] instead.
	//
	//go:fix inline
	func WithVersionFromEnv() Opt {
		return WithAPIVersionFromEnv()
	}

### Usage

Try running "go fix" on this package:

	go fix -mod=readonly ./...
	# github.com/moby/moby/client/internal/gofix/f
	# [github.com/moby/moby/client/internal/gofix/f]
	fix: applied 2 of 3 fixes; 1 file updated. (Re-run the command to apply more.)
	# github.com/moby/moby/client/internal/gofix/g
	# [github.com/moby/moby/client/internal/gofix/g]
	fix: applied 2 of 3 fixes; 2 files updated. (Re-run the command to apply more.)
	# github.com/moby/moby/client/internal/gofix/h
	# [github.com/moby/moby/client/internal/gofix/h]
	fix: applied 1 of 2 fixes; 1 file updated. (Re-run the command to apply more.)
	# github.com/moby/moby/client/internal/gofix/i
	# [github.com/moby/moby/client/internal/gofix/i]
	fix: applied 1 of 3 fixes; 1 file updated. (Re-run the command to apply more.)
	# github.com/moby/moby/client/internal/gofix/j
	# [github.com/moby/moby/client/internal/gofix/j]
	fix: applied 1 of 3 fixes; 1 file updated. (Re-run the command to apply more.)
	# github.com/moby/moby/client/internal/gofix/k_test
	# [github.com/moby/moby/client/internal/gofix/k_test]
	fix: applied 1 of 2 fixes; 1 file updated. (Re-run the command to apply more.)

Or (showing the diff);

	go fix -mod=readonly -diff ./... 2> /dev/null
	--- /go/src/example/a/example_test.go (old)
	+++ /go/src/example/a/example_test.go (new)
	@@ -4,19 +4,18 @@
	 import (
		"context"

	-	"github.com/moby/moby/api/types/container"
	-	"github.com/moby/moby/api/types/strslice"
	+	"github.com/moby/moby/api/types/container"
		"github.com/moby/moby/client"
	 )

	 func OK() {
		opts := []client.Opt{
			client.FromEnv,
	-		client.WithVersion("1.38"),
	+		client.WithAPIVersion("1.38"),
	-		client.WithVersionFromEnv(),
	+		client.WithAPIVersionFromEnv(),
		}

	-	c, err := client.NewClientWithOpts(opts...)
	+	c, err := client.New(opts...)
		if err != nil {
			return
		}
	@@ -24,7 +15,7 @@
		_, _ = c.ContainerCreate(context.Background(), client.ContainerCreateOptions{
			Config: &container.Config{
				Image: "busybox",
	-			Cmd:   strslice.StrSlice{"top"},
	+			Cmd:   []string{"top"},
			},
		})
		_ = c.Close()
	--- /go/src/example/b/example_test.go (old)
	+++ /go/src/example/b/example_test.go (new)
	@@ -4,5 +4,5 @@
	 import "github.com/moby/moby/client"

	 func OK() {
	-	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"))
	+	_, _ = client.New(client.FromEnv, client.WithAPIVersion("1.38"))
	 }
	--- /go/src/example/c/example_test.go (old)
	+++ /go/src/example/c/example_test.go (new)
	@@ -4,5 +4,5 @@
	 import "github.com/moby/moby/client"

	 func OK() {
	-	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"), client.WithVersionFromEnv())
	+	_, _ = client.New(client.FromEnv, client.WithAPIVersion("1.38"), client.WithAPIVersionFromEnv())
	 }
	--- /go/src/example/d/example_test.go (old)
	+++ /go/src/example/d/example_test.go (new)
	@@ -4,5 +4,5 @@
	 import "github.com/moby/moby/client"

	 func OK() {
	-	_, _ = client.NewClientWithOpts(client.FromEnv)
	+	_, _ = client.New(client.FromEnv)
	 }
	--- /go/src/example/e/example_test.go (old)
	+++ /go/src/example/e/example_test.go (new)
	@@ -4,8 +4,7 @@
	 import (
		"context"

	-	"github.com/moby/moby/api/types/container"
	-	"github.com/moby/moby/api/types/strslice"
	+	"github.com/moby/moby/api/types/container"
		"github.com/moby/moby/client"
	 )

	@@ -18,7 +17,7 @@
		_, _ = c.ContainerCreate(context.Background(), client.ContainerCreateOptions{
			Config: &container.Config{
				Image: "busybox",
	-			Cmd:   strslice.StrSlice{"top"},
	+			Cmd:   []string{"top"},
			},
		})
		_ = c.Close()
	--- /go/src/example/k/example1_test.go (old)
	+++ /go/src/example/k/example1_test.go (new)
	@@ -5,5 +5,5 @@

	 // OK works and is in a different package than [KO] (which is in "main_test")
	 func OK() {
	-	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"))
	+	_, _ = client.New(client.FromEnv, client.WithAPIVersion("1.38"))
	 }

### Results

- a: works: multiple inlines, but none are "nested".
- b: works: inlines a single option.
- c: works: inlines multiple options.
- d: works: inlines "outer" method, no deprecated options used.
- f: fails: combines "b" ("OK") and "d" ("KO") in same package.
- g: fails: same as f, but in separate files.
- h: fails; likely due to nested inline.
- i: fails; same as h, but with multiple inlines.
- j: fails; same as i, but formatted over multiple lines.
- k: partially fails; same as g, but using two packages ("main", and "main_test").

Failing cases don't resolve themselves when running "go fix" multiple times,
likely because they're not related to "conflicts".

*/

package gofix
