# Open Container Initiative Runtime Specification

The [Open Container Initiative](http://www.opencontainers.org/) develops specifications for standards on Operating System process and application containers.


Table of Contents

- [Introduction](README.md)
  - [Code of Conduct](#code-of-conduct)
  - [Container Principles](principles.md)
  - [Style and Conventions](style.md)
  - [Roadmap](ROADMAP.md)
  - [Implementations](implementations.md)
  - [project](project.md)
- [Filesystem Bundle](bundle.md)
- Runtime and Lifecycle
  - [General Runtime and Lifecycle](runtime.md)
  - [Linux-specific Runtime and Lifecycle](runtime-linux.md)
- Configuration
  - [General Configuration](config.md)
  - [Linux-specific Configuration](config-linux.md)
  - [Solaris-specific Configuration](config-solaris.md)
  - [Windows-specific Configuration](config-windows.md)
- [Glossary](glossary.md)

In the specifications in the above table of contents, the keywords "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" are to be interpreted as described in [RFC 2119](http://tools.ietf.org/html/rfc2119) (Bradner, S., "Key words for use in RFCs to Indicate Requirement Levels", BCP 14, RFC 2119, March 1997).

The keywords "unspecified", "undefined", and "implementation-defined" are to be interpreted as described in the [rationale for the C99 standard][c99-unspecified].

An implementation is not compliant for a given CPU architecture if it fails to satisfy one or more of the MUST, REQUIRED, or SHALL requirements for the protocols it implements.
An implementation is compliant for a given CPU architecture if it satisfies all the MUST, REQUIRED, and SHALL requirements for the protocols it implements.

Protocols defined by this specification are:
* Linux containers: [runtime.md](runtime.md), [config.md](config.md), [config-linux.md](config-linux.md), and [runtime-linux.md](runtime-linux.md).
* Solaris containers: [runtime.md](runtime.md), [config.md](config.md), and [config-solaris.md](config-solaris.md).
* Windows containers: [runtime.md](runtime.md), [config.md](config.md), and [config-windows.md](config-windows.md).

# Use Cases

To provide context for users the following section gives example use cases for each part of the spec.

#### Application Bundle Builders

Application bundle builders can create a [bundle](bundle.md) directory that includes all of the files required for launching an application as a container.
The bundle contains an OCI [configuration file](config.md) where the builder can specify host-independent details such as [which executable to launch](config.md#process-configuration) and host-specific settings such as [mount](config.md#mounts) locations, [hook](config.md#hooks) paths, Linux [namespaces](config-linux.md#namespaces) and [cgroups](config-linux.md#control-groups).
Because the configuration includes host-specific settings, application bundle directories copied between two hosts may require configuration adjustments.

#### Hook Developers

[Hook](config.md#hooks) developers can extend the functionality of an OCI-compliant runtime by hooking into a container's lifecycle with an external application.
Example use cases include sophisticated network configuration, volume garbage collection, etc.

#### Runtime Developers

Runtime developers can build runtime implementations that run OCI-compliant bundles and container configuration, containing low-level OS and host specific details, on a particular platform.

# Releases

There is a loose [Road Map](./ROADMAP.md).
During the `0.x` series of OCI releases we make no backwards compatibility guarantees and intend to break the schema during this series.

# Contributing

Development happens on GitHub for the spec.
Issues are used for bugs and actionable items and longer discussions can happen on the [mailing list](#mailing-list).

The specification and code is licensed under the Apache 2.0 license found in the [LICENSE](./LICENSE) file.

## Code of Conduct

Participation in the OpenContainers community is governed by [OpenContainer's Code of Conduct](https://github.com/opencontainers/tob/blob/d2f9d68c1332870e40693fe077d311e0742bc73d/code-of-conduct.md).

## Discuss your design

The project welcomes submissions, but please let everyone know what you are working on.

Before undertaking a nontrivial change to this specification, send mail to the [mailing list](#mailing-list) to discuss what you plan to do.
This gives everyone a chance to validate the design, helps prevent duplication of effort, and ensures that the idea fits.
It also guarantees that the design is sound before code is written; a GitHub pull-request is not the place for high-level discussions.

Typos and grammatical errors can go straight to a pull-request.
When in doubt, start on the [mailing-list](#mailing-list).

## Weekly Call

The contributors and maintainers of all OCI projects have a weekly meeting Wednesdays at 2:00 PM (USA Pacific).
Everyone is welcome to participate via [UberConference web][UberConference] or audio-only: 415-968-0849 (no PIN needed.)
An initial agenda will be posted to the [mailing list](#mailing-list) earlier in the week, and everyone is welcome to propose additional topics or suggest other agenda alterations there.
Minutes are posted to the [mailing list](#mailing-list) and minutes from past calls are archived to the [wiki](https://github.com/opencontainers/runtime-spec/wiki) for those who are unable to join the call.

## Mailing List

You can subscribe and join the mailing list on [Google Groups](https://groups.google.com/a/opencontainers.org/forum/#!forum/dev).

## IRC

OCI discussion happens on #opencontainers on Freenode ([logs][irc-logs]).

## Git commit

### Sign your work

The sign-off is a simple line at the end of the explanation for the patch, which certifies that you wrote it or otherwise have the right to pass it on as an open-source patch.
The rules are pretty simple: if you can certify the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe@gmail.com>

using your real name (sorry, no pseudonyms or anonymous contributions.)

You can add the sign off when creating the git commit via `git commit -s`.

### Commit Style

Simple house-keeping for clean git history.
Read more on [How to Write a Git Commit Message](http://chris.beams.io/posts/git-commit/) or the Discussion section of [`git-commit(1)`](http://git-scm.com/docs/git-commit).

1. Separate the subject from body with a blank line
2. Limit the subject line to 50 characters
3. Capitalize the subject line
4. Do not end the subject line with a period
5. Use the imperative mood in the subject line
6. Wrap the body at 72 characters
7. Use the body to explain what and why vs. how
  * If there was important/useful/essential conversation or information, copy or include a reference
8. When possible, one keyword to scope the change in the subject (i.e. "README: ...", "runtime: ...")

[c99-unspecified]: http://www.open-std.org/jtc1/sc22/wg14/www/C99RationaleV5.10.pdf#page=18
[UberConference]: https://www.uberconference.com/opencontainers
[irc-logs]: http://ircbot.wl.linuxfoundation.org/eavesdrop/%23opencontainers/
