# App Container Specification

_For version information, see [VERSION](VERSION)_

The "App Container" defines an image format, image discovery mechanism and execution environment that can exist in several independent implementations. The core goals include:

* Design for fast downloads and starts of the containers
* Ensure images are cryptographically verifiable and highly-cacheable
* Design for composability and independent implementations
* Use common technologies for crypto, archive, compression and transport
* Use the DNS namespace to name and discover container images

To achieve these goals this specification is split out into a number of smaller sections.

1. The **[App Container Image](#app-container-image)** defines: how files are assembled together into a single image, verified on download and placed onto disk to be run.

2. The **[App Container Executor](#app-container-executor)** defines: how an app container image on disk is run and the environment it is run inside including cgroups, namespaces and networking.

    * The [Metadata Server](#app-container-metadata-service) defines how a container can introspect and get a cryptographically verifiable identity from the execution environment.

3. The **[App Container Image Discovery](#app-container-image-discovery)** defines: how to take a name like example.com/reduce-worker and translate that into a downloadable image.


## Example Use Case

To provide context to the specs outlined below we will walk through an example.

A user wants to launch a container running three processes.
The three processes the user wants to run are the apps named `example.com/reduce-worker-register`, `example.com/reduce-worker`, and `example.com/reduce-backup`.
First, the executor will check the cache and find that it doesn't have images available for these apps.
So, it will make an HTTPS request to example.com and using the `<meta>` tags there finds that the containers can be found at:

	https://storage-mirror.example.com/reduce-worker.aci
	https://storage-mirror.example.com/worker-backup.aci
	https://storage-mirror.example.com/reduce-worker-register.aci

The executor downloads these three images and puts them into its local on-disk cache.
Then the executor extracts three fresh copies of the images to create instances of the "on-disk app format" and reads the three image manifests to figure out what binaries will need to be executed.

Based on user input, the executor now sets up the necessary cgroups, network interfaces, etc. and runs the `pre-start` event handlers for each app.
Next, it forks the `reduce-worker`, `worker-backup`, and `register` processes in their shared namespaces inside the container, chrooted into their respective root filesystems.

At some point, the container will get some notification that it needs to stop.
The executor will send `SIGTERM` to the processes and after they have exited the `post-stop` event handlers for each app will run.

Now, let's dive into the pieces that took us from three URLs to a running container on our system.

## App Container Image

An *App Container Image* (ACI) contains all files and metadata needed to execute a given app.
In some ways an ACI can be thought of as equivalent to a static binary.

### Image Layout

The on-disk layout of an app container is straightforward.
It includes a *rootfs* with all of the files that will exist in the root of the app, and an *app image manifest* describing the contents of the image and how to execute the app.

```
/manifest
/rootfs
/rootfs/usr/bin/data-downloader
/rootfs/usr/bin/reduce-worker
```

### Image Archives

The ACI file format ("image archive") aims for flexibility and relies on standard and common technologies: HTTP, gpg, tar and gzip.
This set of formats makes it easy to build, host and secure a container using technologies that are widely available and battle-tested.

- Image archives MUST be a tar formatted file.
- All files in the image MUST maintain all of their original properties, including timestamps, Unix modes, and extended attributes (xattrs).
- Image archives MUST be named with the suffix `.aci`, irrespective of compression/encryption (see below).
- Image archives SHOULD be signed using PGP, the format MUST be ascii-armored detached signature mode.
- Image signatures MUST be named with the suffix `.aci.asc`.

There are two further transformations that may be applied to image archives for transport:
- Image archives MAY be compressed with `gzip`, `bzip2`, or `xz`.
- Image archives MAY be encrypted with AES symmetric encryption, after (optional) compression.

The following example demonstrates the creation of a simple ACI using common command-line tools.
In this case, the ACI is both compressed and encrypted.

```
tar cvf reduce-worker.tar manifest rootfs
gpg --armor --output reduce-worker.aci.asc --detach-sig reduce-worker.tar
gzip reduce-worker.tar -c > reduce-worker.aci
gpg --output reduce-worker.aci --digest-algo sha256 --cipher-algo AES256 --symmetric reduce-worker.aci
```

**Note**: the key distribution mechanism to facilitate image signature validation is not defined here.
Implementations of the app container spec will need to provide a mechanism for users to configure the list of signing keys to trust, or use the key discovery described in [App Container Image Discovery](#app-container-image-discovery).

An example application container image builder is [actool](https://github.com/appc/spec/tree/master/actool).

### Image ID

An image is addressed and verified against the hash of its uncompressed tar file, the _image ID_.
The image ID provides a way to uniquely and globally reference an image, and verify its integrity at any point.
The default digest format is sha512, but all hash IDs in this format are prefixed by the algorithm used (e.g. sha512-a83...).

```
echo sha512-$(sha512sum reduce-worker.tar | awk '{print $1}')
```

### Image Manifest

The [image manifest](#image-manifest-schema) is a [JSON](https://tools.ietf.org/html/rfc4627) file that includes details about the contents of the ACI, and optionally information about how to execute a process inside the ACI's rootfs.
If included, execution details include mount points that should exist, the user, the command args, default cgroup settings and more.
The manifest MAY also define binaries to execute in response to lifecycle events of the main process such as *pre-start* and *post-stop*.

Image manifests MUST be valid JSON located in the file `manifest` in the root of the image archive.
Image manifests MAY specify dependencies, which describe how to assemble the final rootfs from a collection of other images.
As an example, an app might require special certificates to be layered into its filesystem.
In this case, the app can reference the name "example.com/trusted-certificate-authority" as a dependency in the image manifest.
The dependencies are applied in order and each image dependency can overwrite files from the previous dependency.
An optional *path whitelist* can be provided, in which case all non-specified files from all dependencies will be omitted in the final, assembled rootfs.

Image Format TODO

* Define the garbage collection lifecycle of the container filesystem including:
    * Format of app exit code and signal
    * The refcounting plan for resources consumed by the ACE such as volumes
* Define the lifecycle of the container as all exit or first to exit
* Define security requirements for a container. In particular is any isolation of users required between containers? What user does each application run under and can this be root (i.e. "real" root in the host).
* Define how apps are supposed to communicate; can they/do they 'see' each other (a section in the apps perspective would help)?


## App Container Executor

App Containers are a combination of a number of technologies which are not aware of each other.
This specification attempts to define a reasonable subset of steps to accomplish a few goals:

* Creating a filesystem hierarchy in which the app will execute
* Running the app process inside of a combination of resource and namespace isolations
* Executing the application inside of this environment

There are two "perspectives" in this process.
The "*executor*" perspective consists of the steps that the container executor must take to set up the containers. The "*app*" perspective is how the app processes inside the container see the environment.

### Executor Perspective

#### Filesystem Setup

Every execution of an app container should start from a clean copy of the app image.
The simplest implementation will take an application container image (ACI) and extract it into a new directory:

```
cd $(mktemp -d -t temp.XXXX)
mkdir hello
tar xzvf /var/lib/pce/hello.aci -C hello
```

Other implementations could increase performance and de-duplicate data by building on top of overlay filesystems, copy-on-write block devices, or a content-addressed file store.
These details are orthogonal to the runtime environment.

#### Container Runtime Manifest

A container executes one or more apps with shared PID namespace, network namespace, mount namespace, IPC namespace and UTS namespace.
Each app will start chrooted into its own unique read-write rootfs before execution.
The definition of the container is a list of apps that should be launched together, along with isolators that should apply to the entire container.
This is codified in a [Container Runtime Manifest](#container-runtime-manifest-schema).

This example container will use a set of three apps:

| Name                               | Version | Image hash                                      |
|------------------------------------|---------|-------------------------------------------------|
| example.com/reduce-worker          | 1.0.0   | sha512-277205b3ae3eb3a8e042a62ae46934b470e43... |
| example.com/worker-backup          | 1.0.0   | sha512-3e86b59982e49066c5d813af1c2e2579cbf57... |
| example.com/reduce-worker-register | 1.0.0   | sha512-86298e1fdb95ec9a45b5935504e26ec29b8fe... |

#### Volume Setup

Volumes that are specified in the Container Runtime Manifest are mounted into each of the apps via a bind mount.
For example say that the worker-backup and reduce-worker both have a MountPoint named "work".
In this case, the container executor will bind mount the host's `/opt/tenant1/database` directory into the Path of each of the matching "work" MountPoints of the two containers.

#### Network Setup

An App Container must have a [layer 3](http://en.wikipedia.org/wiki/Network_layer) (commonly called the IP layer) network interface, which can be instantiated in any number of ways (e.g. veth, macvlan, ipvlan, device pass-through).
The network interface should be configured with an IPv4/IPv6 address that is reachable from other containers.

#### Logging

Apps should log to stdout and stderr. The container executor is responsible for capturing and persisting the output.

If the application detects other logging options, such as the /run/systemd/system/journal socket, it may optionally upgrade to using those mechanisms.
Note that logging mechanisms other than stdout and stderr are not required by this specification (or tested by the compliance tests).

### Apps Perspective

#### Execution Environment

* **Working directory** defaults to the root of the application image, overridden with "workingDirectory"
* **PATH** `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
* **USER, LOGNAME** username of the user executing this app
* **HOME** home directory of the user
* **SHELL** login shell of the user
* **AC_APP_NAME** name of the application, as defined in the image manifest
* **AC_METADATA_URL** URL where the metadata service for this container can be found

### Isolators

Isolators enforce resource constraints rather than namespacing.
Isolators may be applied to individual applications, to whole containers, or to both.
Some well known isolators can be verified by the specification.
Additional isolators will be added to this specification over time.

|Name|Type|Schema|Example|
|---------------------------|------|------------------------------------|------------------------------------|
|cpu/shares                 |string|"&lt;uint&gt;"                      |"4096"                              |
|memory/limit               |string|"&lt;bytes&gt;"                     |"1G", "5T", "4K"                    |
|cpu/mask                   |string|"&lt;cpu range&gt;"                 |"0", "0-3", "0,2,4-7"               |
|block-io/read-bandwidth    |string|"&lt;path to file&gt; &lt;bytes&gt;"|"/tmp 1K"                           |
|block-io/write-bandwidth   |string|"&lt;path to file&gt; &lt;bytes&gt;"|"/tmp 1K"                           |
|network-io/read-bandwidth  |string|"&lt;device name&gt; &lt;bytes&gt;" |"eth0 100M"                         |
|network-io/write-bandwidth |string|"&lt;device name&gt; &lt;bytes&gt;" |"eth0 100M"                         |
|capabilities/bounding-set  |string|"&lt;cap&gt; &lt;cap&gt; ..."       |"CAP_NET_BIND_SERVICE CAP_SYS_ADMIN"|

#### Types

* uint: base 10 formatted unsigned int as a string
* bytes: Suffix to a base 10 int to make it a K, M, G, or T for base 1024

## App Container Image Discovery

An app name has a URL-like structure, for example `example.com/reduce-worker`.
However, there is no scheme on this app name, so it cannot be directly resolved to an app container image URL.
Furthermore, attributes other than the name may be required to unambiguously identify an app (version, OS and architecture).
App Container Image Discovery prescribes a discovery process to retrieve an image based on the app name and these attributes.
Image Discovery is inspired by Go's [remote import paths](https://golang.org/cmd/go/#hdr-Remote_import_paths).

### Simple Discovery

First, try to fetch the app container image by rendering the following template and directly retrieving the resulting URL:

    https://{name}-{version}-{os}-{arch}.aci

For example, given the app name `example.com/reduce-worker`, with version `1.0.0`, arch `amd64`, and os `linux`, try to retrieve:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci

If this fails, move on to meta discovery.
If this succeeds, try fetching the signature using the same template but with a `.aci.asc` extension:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci.asc

### Meta Discovery

If simple discovery fails, then we use HTTPS+HTML meta tags to resolve an app name to a downloadable URL.
For example, if the ACE is looking for `example.com/reduce-worker` it will request:

    https://example.com/reduce-worker?ac-discovery=1

Then inspect the HTML returned for meta tags that have the following format:

```
<meta name="ac-discovery" content="prefix-match url-tmpl">
<meta name="ac-discovery-pubkeys" content="prefix-match url">
```

* `ac-discovery` should contain a URL template that can be rendered to retrieve the ACI or signature
* `ac-discovery-pubkeys` should contain a URL that provides a set of public keys that can be used to verify the signature of the ACI

Some examples for different schemes and URLs:

```
<meta name="ac-discovery" content="example.com https://storage.example.com/{os}/{arch}/{name}-{version}.{ext}?torrent">
<meta name="ac-discovery" content="example.com hdfs://storage.example.com/{name}-{version}-{os}-{arch}.{ext}">
<meta name="ac-discovery-pubkeys" content="example.com https://example.com/pubkeys.gpg">
```

The algorithm first ensures that the prefix of the AC Name matches the prefix-match and then if there is a match it will request the equivalent of:

```
curl $(echo "$urltmpl" | sed -e "s/{name}/$appname/" -e "s/{version}/$version/" -e "s/{os}/$os/" -e "s/{arch}/$arch/" -e "s/{ext}/$ext/")
```

where _appname_, _version_, _os_, and _arch_ are set to their respective values for the application, and _ext_ is either `aci` or `aci.asc` for retrieving an app container image or signature respectively.

In our example above this would be:

```
sig: https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci.asc
aci: https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci
keys: https://example.com/pubkeys.gpg
```

This mechanism is only used for discovery of contents URLs.

If the first attempt at fetching the discovery URL returns a status code other than `200 OK`, `3xx`, or does not contain any `ac-discovery` meta tags then the next higher path in the name should be tried.
For example if the user has `example.com/project/subproject` and we first try `example.com/project/subproject` but don't find a meta tag then try `example.com/project` then try `example.com`.

All HTTP redirects should be followed when the discovery URL returns a `3xx` status code.

Discovery URLs that require interpolation are [RFC6570](https://tools.ietf.org/html/rfc6570) URI templates.

### Validation

Implementations of the spec should enforce any signature validation rules set in place by the operator.
For example, in a testing environment, signature validation might be disabled, in which case the implementation would omit the signature retrieval.

Implementations must ensure that the name in the Image Manifest in the retrieved ACI matches the initial name used for discovery.

### Authentication

Authentication during the discovery process is optional.
If an attempt at fetching any resource (the initial discovery URL, an app container image, or signature) returns a `401 Unauthorized`, implementations should enact the authentication policy set by the operator.
For example, some implementations might only perform HTTP basic authentication over HTTPS connections.

## App Container Metadata Service

For a variety of reasons, it is desirable to not write files to the filesystem in order to run a container:
* Secrets can be kept outside of the container (such as the identity endpoint specified below)
* Writing files leads to assumptions like a libc environment attempting parse `/etc/hosts`
* The container can be run on top of a cryptographically secured read-only filesystem
* Metadata is a proven system for virtual machines

The app container specification defines an HTTP-based metadata service for providing metadata to containers.

### Metadata Server

The ACE must provide a Metadata server on the address given to the container via the `AC_METADATA_URL` environment variable.

Clients querying any of these endpoints must specify the `Metadata-Flavor: AppContainer` header.

### Container Metadata

Information about the container that this app is executing in.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/container`

| Entry       | Description |
|-------------|-------------|
|annotations/ | Top level annotations from container runtime manifest. |
|manifest     | The container manifest JSON. |
|uid          | The unique execution container uid. |

### App Metadata

Every running process will be able to introspect its App Name via the `AC_APP_NAME` environment variable.
This is necessary to query for the correct endpoint metadata.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/apps/$AC_APP_NAME/`

| Entry         | Description |
|---------------|-------------|
|annotations/   | Annotations from image manifest merged with app annotations from container runtime manifest. |
|image/manifest | The original manifest file of the app. |
|image/id       | Image ID (digest) this app is contained in. |

### Identity Endpoint

As a basic building block for building a secure identity system, the metadata service must provide an HMAC (described in [RFC2104](https://www.ietf.org/rfc/rfc2104.txt)) endpoint for use by the apps in the container.
This gives a cryptographically verifiable identity to the container based on its container unique ID and the container HMAC key, which is held securely by the ACE.

Accessible at `$AC_METADATA_URL/acMetadata/v1/container/hmac`

| Entry | Description |
|-------|-------------|
|sign   | POST a form with content=&lt;object to sign&gt; and retrieve a base64 hmac-sha512 signature as the response body. The metadata service holds onto the secret key as a sort of container TPM. |
|verify | Verify a signature from another container. POST a form with content=&lt;object that was signed&gt;, uid=&lt;uid of the container that generated the signature&gt;, signature=&lt;base64 encoded signature&gt;. Returns 200 OK if the signature passes and 403 Forbidden if the signature check fails. |


## AC Name Type

An AC Name Type is restricted to lowercase characters accepted by the DNS [RFC](http://tools.ietf.org/html/rfc1123#page-13) and "/".

Examples:

* database
* example.com/database
* example.com/ourapp
* sub-domain.example.com/org/product/release

An AC Name Type cannot be an empty string.
The AC Name Type is used as the primary key for a number of fields in the schemas below.
The schema validator will ensure that the keys conform to these constraints.


## Manifest Schemas

### Image Manifest Schema

JSON Schema for the Image Manifest (app image manifest, ACI manifest), conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```
{
    "acKind": "ImageManifest",
    "acVersion": "0.3.0",
    "name": "example.com/reduce-worker",
    "labels": [
        {
            "name": "version",
            "value": "1.0.0"
        },
        {
            "name": "arch",
            "value": "amd64"
        },
        {
            "name": "os",
            "value": "linux"
        }
    ],
    "app": {
        "exec": [
            "/usr/bin/reduce-worker",
            "--quiet"
        ],
        "user": "100",
        "group": "300",
        "eventHandlers": [
            {
                "exec": [
                    "/usr/bin/data-downloader"
                ],
                "name": "pre-start"
            },
            {
                "exec": [
                    "/usr/bin/deregister-worker",
                    "--verbose"
                ],
                "name": "post-stop"
            }
        ],
        "workingDirectory": "/opt/work",
        "environment": [
            {
                "name": "REDUCE_WORKER_DEBUG",
                "value": "true"
            }
        ],
        "isolators": [
            {
                "name": "cpu/shares",
                "value": "20"
            },
            {
                "name": "memory/limit",
                "value": "1G"
            },
            {
                "name": "cpu/mask",
                "value": "0-3"
            },
            {
                "name": "capabilities/bounding-set",
                "value": "CAP_NET_BIND_SERVICE CAP_SYS_ADMIN"
            }
        ],
        "mountPoints": [
            {
                "name": "work",
                "path": "/var/lib/work",
                "readOnly": false
            }
        ],
        "ports": [
            {
                "name": "health",
                "port": 4000,
                "protocol": "tcp",
                "socketActivated": true
            }
        ]
    },
    "dependencies": [
        {
            "app": "example.com/reduce-worker-base",
            "imageID": "sha512-...",
            "labels": [
                {
                    "name": "os",
                    "value": "linux"
                },
                {
                    "name": "env",
                    "value": "canary"
                }
            ]
        }
    ],
    "pathWhitelist": [
        "/etc/ca/example.com/crt",
        "/usr/bin/map-reduce-worker",
        "/opt/libs/reduce-toolkit.so",
        "/etc/reduce-worker.conf",
        "/etc/systemd/system/"
    ],
    "annotations": [
        {
            "name": "authors",
            "value": "Carly Container <carly@example.com>, Nat Network <[nat@example.com](mailto:nat@example.com)>"
        },
        {
            "name": "created",
            "value": "2014-10-27T19:32:27.67021798Z"
        },
        {
            "name": "documentation",
            "value": "https://example.com/docs"
        },
        {
            "name": "homepage",
            "value": "https://example.com"
        }
    ]
}
```

* **acKind** (required) must be set to "ImageManifest"
* **acVersion** (required) represents the version of the schema specification that the manifest implements (string, must be in [semver](http://semver.org/) format)
* **name** (required) is used as a human readable index to the container image. (string, restricted to the AC Name formatting)
* **labels** (optional) list of label objects (where the *name* is restricted to the AC Name formatting and *value* is an arbitrary string). Label names must be unique within the list, and (to avoid confusion with the image's name) cannot be "name". Labels are used during image discovery and dependency resolution. Several well-known labels are defined:
    * **version** when combined with "name", this should be unique for every build of an app (on a given "os"/"arch" combination).
    * **os**, **arch** can together be considered to describe the syscall ABI this image requires. **arch** is meaningful only if **os** is provided. If one or both values are not provided, the image is assumed to be OS- and/or architecture-independent. Currently supported combinations are listed in the [`types.ValidOSArch`](schema/types/labels.go) variable, which can be updated by an implementation that supports other combinations. The combinations whitelisted by default are (in format `os/arch`): `linux/amd64`, `linux/i386`, `freebsd/amd64`, `freebsd/i386`, `freebsd/arm`, `darwin/x86_64`, `darwin/i386`.
* **app** (optional) if present, defines the default parameters that can be used to execute this image as an application.
    * **exec** (required) executable to launch and any flags (array of strings, must be non-empty; the executable must be an absolute path within the app rootfs; ACE can append or override)
    * **user**, **group** (required) indicate either the UID/GID or the username/group name the app should run as inside the container (freeform string). If the user or group field begins with a "/", the owner and group of the file found at that absolute path inside the rootfs is used as the UID/GID of the process.
    * **eventHandlers** (optional) list of eventHandler objects. eventHandlers allow the app to have several hooks based on lifecycle events. For example, you may want to execute a script before the main process starts up to download a dataset or backup onto the filesystem. An eventHandler is a simple object with two fields - an **exec** (array of strings, ACE can append or override), and a **name** (there may be only one eventHandler of a given name), which must be one of:
        * **pre-start** - executed and must exit before the long running main **exec** binary is launched
        * **post-stop** - executed if the main **exec** process is killed. This can be used to cleanup resources in the case of clean application shutdown, but cannot be relied upon in the face of machine failure.
    * **workingDirectory** (optional) working directory of the launched application, relative to the application image's root (must be an absolute path, defaults to "/", ACE can override). If the directory does not exist in the application's assembled rootfs (including any dependent images and mounted volumes), the ACE must fail execution.
    * **environment** (optional) a list of name / value objects representing the app's environment variables (ACE can append). The **name** must consist solely of letters, digits, and underscores '_' as outlined in [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html).
    * **mountPoints** (optional) locations where a container is expecting external data to be mounted. The name indicates an executor-defined label to look up a mount point, and the path stipulates where it should actually be mounted inside the rootfs. The name is restricted to the AC Name Type formatting. "readOnly" should be a boolean indicating whether or not the mount point should be read-only (defaults to "false" if unsupplied).
    * **ports** (optional) protocols and port numbers that the container will be listening on once started. The key is restricted to the AC Name formatting. This information is primarily informational to help the user find ports that are not well known. It could also optionally be used to limit the inbound connections to the container via firewall rules to only ports that are explicitly exposed.
        * **socketActivated** (optional) if set to true, the application expects to be [socket activated](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) on these ports. The ACE must pass file descriptors using the [socket activation protocol](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) that are listening on these ports when starting this container. If multiple apps in the same container are using socket activation then the ACE must match the sockets to the correct apps using getsockopt() and getsockname().
    * **isolators** (optional) list of well-known isolation steps that should be applied to the app. **name** is restricted to the [AC Name](#ac-name-type) formatting and **value** can be a freeform string. Any isolators specified in the Image Manifest can be overridden at runtime via the Container Runtime Manifest. The executor can either ignore isolator keys it does not understand or error. In practice this means there might be certain isolators (for example, an AppArmor policy) that an executor doesn't understand so it will simply skip that entry.
* **dependencies** (optional) list of dependent application images that need to be placed down into the rootfs before the files from this image (if any). The ordering is significant. See [Dependency Matching](#dependency-matching) for how dependencies should be retrieved.
    * **app** (required) name of the dependent app container image.
    * **imageID** (optional) content hash of the dependency. If provided, the retrieved dependency must match the hash. This can be used to produce deterministic, repeatable builds of an App Image that has dependencies.
    * **labels** (optional) list of label objects of the same form as in the top level ImageManifest. See [Dependency Matching](#dependency-matching) for how these are used.
* **pathWhitelist** (optional, list of strings) complete whitelist of paths that should exist in the rootfs after assembly (i.e. unpacking the files in this image and overlaying its dependencies, in order). Paths that end in slash will ensure the directory is present but empty. This field is only required if the app has dependencies and you wish to remove files from the rootfs before running the container; an empty value means that all files in this image and any dependencies will be available in the rootfs.
* **annotations** (optional) list of annotation objects (where the *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string). Annotation names must be unique within the list. Annotations can be used by systems outside of the ACE (ACE can override). If you are defining new annotations, please consider submitting them to the specification. If you intend for your field to remain special to your application please be a good citizen and prefix an appropriate namespace to your key names. Recognized annotations include:
    * **created** date on which this container was built (string, must be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format)
    * **authors** contact details of the people or organization responsible for the containers (freeform string)
    * **homepage** URL to find more information on the container (string, must be a URL with scheme HTTP or HTTPS)
    * **documentation** URL to get documentation on this container (string, must be a URL with scheme HTTP or HTTPS)

#### Dependency Matching

Dependency matching is based on a combination of the three different fields of the dependency - **app**, **imageID**, and **labels**.
First, the image discovery mechanism is used to locate a dependency.
If any labels are specified in the dependency, they are passed to the image discovery mechanism, and should be used when locating the image.

If the image discovery process successfully returns an image, it will be compared as follows
If the dependency specification has an image ID, it will be compared against the hash of image returned, and must match.
Otherwise, the labels in the dependency specification are compared against the labels in the retrieved ACI (i.e. in its ImageManifest), and must match.
A label is considered to match if it meets one of three criteria:
- It is present in the dependency specification and present in the dependency's ImageManifest with the same value.
- It is absent from the dependency specification and present in the dependency's ImageManifest, with any value.
This facilitates "wildcard" matching and a variety of common usage patterns, like "noarch" or "latest" dependencies.
For example, an AppImage containing a set of bash scripts might omit both "os" and "arch", and hence could be used as a dependency by a variety of different AppImages.
Alternatively, an AppImage might specify a dependency with no image ID and no "version" label, and the image discovery mechanism could always retrieve the latest version of an AppImage

### Container Runtime Manifest Schema

JSON Schema for the Container Runtime Manifest (container manifest), conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```
{

    "acVersion": "0.3.0",
    "acKind": "ContainerRuntimeManifest",
    "uuid": "6733C088-A507-4694-AABF-EDBE4FC5266F",
    "apps": [
        {
            "app": "example.com/reduce-worker-1.0.0",
            "imageID": "sha512-...",
            "mounts": [
                 {"volume": "work", "mountPoint": "work"}
            ]
        },
        {
            "app": "example.com/worker-backup-1.0.0",
            "imageID": "sha512-...",
            "isolators": [
                {
                    "name": "memory/limit",
                    "value": "1G"
                }
            ],
            "annotations": [
                {
                    "name": "foo",
                    "value": "baz"
                }
            ],
            "mounts": [
                 {"volume": "work", "mountPoint": "backup"}
            ]
        },
        {
            "app": "example.com/reduce-worker-register-1.0.0",
            "imageID": "sha512-..."
        }
    ],
    "volumes": [
        {
            "name": "work",
            "kind": "host",
            "source": "/opt/tenant1/work",
            "readOnly": true
        }
    ],
    "isolators": [
        {
           "name": "memory/limit",
           "value": "4G"
        }
    ],
    "annotations": [
        {
           "name": "ip-address",
           "value": "10.1.2.3"
        }
    ]
}
```

* **acVersion** (required) represents the version of the schema spec (string, must be in [semver](http://semver.org/) format)
* **acKind** (required) must be set to "ContainerRuntimeManifest"
* **uuid** (required) must be an [RFC4122 UUID](http://www.ietf.org/rfc/rfc4122.txt) that represents this instance of the container (string, must be in [RFC4122](http://www.ietf.org/rfc/rfc4122.txt) format)
* **apps** (required) list of apps that will execute inside of this container
    * **app** (optional) name of the app (string, restricted to AC Name formatting)
    * **imageID** (required) content hash of the image that this app will execute inside of (string, must be of the format "type-value", where "type" is "sha512" and value is the hex encoded string of the hash)
    * **mounts** (optional) list of mounts mapping an app mountPoint to a volume
      * **volume** name of the volume that will fulfill this mount (string, restricted to the AC Name formatting)
      * **mountPoint** name of the app mount point to place the volume on (string, restricted to the AC Name formatting)
    * **isolators** (optional) list of isolators that should be applied to this app (key is restricted to the AC Name formatting and the value can be a freeform string)
    * **annotations** (optional) arbitrary metadata appended to the app. Should be a list of annotation objects (where the *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string). Annotation names must be unique within the list. These will be merged with annotations provided by the image manifest when queried via the metadata service; values in this list take precedence over those in the image manifest.
* **volumes** (optional) list of volumes which should be mounted into each application's filesystem
    * **name** (required) used to map the volume to an app's mountPoint at runtime. (string, restricted to the AC Name formatting)
    * **kind** (required) either "empty" or "host". "empty" fulfills a mount point by ensuring the path exists (writes go to container). "host" fulfills a mount point with a bind mount from a **source**.
    * **source** (required if **kind** is "host") absolute path on host to be bind mounted into the container under a mount point.
    * **readOnly** (optional if **kind** is "host") whether or not the volume should be mounted read only.
* **isolators** (optional) list of isolators that will apply to all apps in this container (name is restricted to the AC Name formatting and the value can be a freeform string)
* **annotations** (optional) arbitrary metadata the executor should make available to applications via the metadata service. Should be a list of annotation objects (where the *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string). Annotation names must be unique within the list.
