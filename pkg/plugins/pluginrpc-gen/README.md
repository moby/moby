Plugin RPC Generator
====================

Generates go code from a Go interface definition for proxying between the plugin
API and the subsystem being extended.

## Usage

Given an interface definition:

```go
type volumeDriver interface {
	Create(name string, opts opts) (err error)
	Remove(name string) (err error)
	Path(name string) (mountpoint string, err error)
	Mount(name string) (mountpoint string, err error)
	Unmount(name string) (err error)
}
```

**Note**: All function options and return values must be named in the definition.

Run the generator:

```bash
$ pluginrpc-gen --type volumeDriver --name VolumeDriver -i volume/drivers/extpoint.go -o volume/drivers/proxy.go
```

Where:
- `--type` is the name of the interface to use
- `--name` is the subsystem that the plugin "Implements"
- `-i` is the input file containing the interface definition
- `-o` is the output file where the generated code should go

**Note**: The generated code will use the same package name as the one defined in the input file

Optionally, you can skip functions on the interface that should not be
implemented in the generated proxy code by passing in the function name to `--skip`.
This flag can be specified multiple times.

You can also add build tags that should be prepended to the generated code by
supplying `--tag`. This flag can be specified multiple times.


## Annotations

`pluginrpc-gen` supports annotations to customize the behavior of the generated code. These annotations are added as comments directly above the interface methods.

### Supported Annotations

1. **`pluginrpc-gen:timeout-type=<value>`**
   - Specifies the timeout type to use for the method in the generated proxy code.
   - The `<value>` can be:
     - `short`: Uses the `shortTimeout` constant (default: 1 minute).
     - `long`: Uses the `longTimeout` constant (default: 2 minutes).

### Usage

To use the `pluginrpc-gen:timeout-type` annotation, place it directly above the method definition in the interface.  
To use the timeout value annotations, place them directly above the interface definition.

#### Example

```go
type volumeDriver interface {
    // pluginrpc-gen:timeout-type=long
    Create(name string, opts map[string]string) error

    // pluginrpc-gen:timeout-type=short
    Remove(name string) error
}
```

### Default Behavior

- If no `pluginrpc-gen:timeout-type` annotation is provided, the `shortTimeout` value is used by default.

## Known issues

## go-generate

You can also use this with go-generate, which is pretty awesome.  
To do so, place the code at the top of the file which contains the interface
definition (i.e., the input file):

```go
//go:generate pluginrpc-gen -i $GOFILE -o proxy.go -type volumeDriver -name VolumeDriver
```

Then cd to the package dir and run `go generate`

**Note**: the `pluginrpc-gen` binary must be within your `$PATH`
