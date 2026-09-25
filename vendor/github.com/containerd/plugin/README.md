# plugin

A Go package providing a common plugin interface across containerd repositories.

This package is intended to be imported by the main containerd repository as well as plugin implementations.
By sharing a common implementations, plugins can register themselves without needing to import the main containerd repository.
This plugin is intended to provide an interface and common functionality, but is not intended to define plugin types used by containerd.
Plugins should copy plugin type strings to avoid creating unintended depdenencies.

## Project details

**plugin** is a containerd sub-project, licensed under the [Apache 2.0 license](./LICENSE).
As a containerd sub-project, you will find the:
 * [Project governance](https://github.com/containerd/project/blob/main/GOVERNANCE.md),
 * [Maintainers](https://github.com/containerd/project/blob/main/MAINTAINERS),
 * and [Contributing guidelines](https://github.com/containerd/project/blob/main/CONTRIBUTING.md)

information in our [`containerd/project`](https://github.com/containerd/project) repository.

