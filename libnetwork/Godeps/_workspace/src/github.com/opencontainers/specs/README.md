# Open Container Specifications

This project is where the [Open Container Initiative](http://www.opencontainers.org/) Specifications are written. This is a work in progress. We should have a first draft by end of July 2015.

Table of Contents

- [Filesystem Bundle](bundle.md)
- [Container Configuration](config.md)
  - [Linux Specific Configuration](config-linux.md)
- [Runtime and Lifecycle](runtime.md)

## Use Cases

To provide context for users the following section gives example use cases for each part of the spec.

### Filesystem Bundle & Configuration

- A user can create a root filesystem and configuration, with low-level OS and host specific details, and launch it as a container under an Open Container runtime.

# The 5 principles of Standard Containers

Define a unit of software delivery called a Standard Container. The goal of a Standard Container is to encapsulate a software component and all its dependencies in a format that is self-describing and portable, so that any compliant runtime can run it without extra dependencies, regardless of the underlying machine and the contents of the container.

The specification for Standard Containers is straightforward. It mostly defines 1) a file format, 2) a set of standard operations, and 3) an execution environment.

A great analogy for this is the shipping container. Just like how Standard Containers are a fundamental unit of software delivery, shipping containers are a fundamental unit of physical delivery.

## 1. Standard operations

Just like shipping containers, Standard Containers define a set of STANDARD OPERATIONS. Shipping containers can be lifted, stacked, locked, loaded, unloaded and labelled. Similarly, Standard Containers can be created, started, and stopped using standard container tools (what this spec is about); copied and snapshotted using standard filesystem tools; and downloaded and uploaded using standard network tools.

## 2. Content-agnostic

Just like shipping containers, Standard Containers are CONTENT-AGNOSTIC: all standard operations have the same effect regardless of the contents. A shipping container will be stacked in exactly the same way whether it contains Vietnamese powder coffee or spare Maserati parts. Similarly, Standard Containers are started or uploaded in the same way whether they contain a postgres database, a php application with its dependencies and application server, or Java build artifacts.

## 3. Infrastructure-agnostic

Both types of containers are INFRASTRUCTURE-AGNOSTIC: they can be transported to thousands of facilities around the world, and manipulated by a wide variety of equipment. A shipping container can be packed in a factory in Ukraine, transported by truck to the nearest routing center, stacked onto a train, loaded into a German boat by an Australian-built crane, stored in a warehouse at a US facility, etc. Similarly, a standard container can be bundled on my laptop, uploaded to S3, downloaded, run and snapshotted by a build server at Equinix in Virginia, uploaded to 10 staging servers in a home-made Openstack cluster, then sent to 30 production instances across 3 EC2 regions.

## 4. Designed for automation

Because they offer the same standard operations regardless of content and infrastructure, Standard Containers, just like their physical counterparts, are extremely well-suited for automation. In fact, you could say automation is their secret weapon.

Many things that once required time-consuming and error-prone human effort can now be programmed. Before shipping containers, a bag of powder coffee was hauled, dragged, dropped, rolled and stacked by 10 different people in 10 different locations by the time it reached its destination. 1 out of 50 disappeared. 1 out of 20 was damaged. The process was slow, inefficient and cost a fortune - and was entirely different depending on the facility and the type of goods.

Similarly, before Standard Containers, by the time a software component ran in production, it had been individually built, configured, bundled, documented, patched, vendored, templated, tweaked and instrumented by 10 different people on 10 different computers. Builds failed, libraries conflicted, mirrors crashed, post-it notes were lost, logs were misplaced, cluster updates were half-broken. The process was slow, inefficient and cost a fortune - and was entirely different depending on the language and infrastructure provider.

## 5. Industrial-grade delivery

There are 17 million shipping containers in existence, packed with every physical good imaginable. Every single one of them can be loaded onto the same boats, by the same cranes, in the same facilities, and sent anywhere in the World with incredible efficiency. It is embarrassing to think that a 30 ton shipment of coffee can safely travel half-way across the World in *less time* than it takes a software team to deliver its code from one datacenter to another sitting 10 miles away.

With Standard Containers we can put an end to that embarrassment, by making INDUSTRIAL-GRADE DELIVERY of software a reality.

# Contributing

Development happens on github for the spec.  Issues are used for bugs and actionable items and longer
discussions can happen on the mailing list.  You can subscribe and join the mailing list on
[google groups](https://groups.google.com/a/opencontainers.org/forum/#!forum/dev).

The specification and code is licensed under the Apache 2.0 license found in 
the `LICENSE` file of this repository.  

## Weekly Call

The contributors and maintainers of the project have a weekly meeting Wednesdays at 10:00 AM PST.
Everyone is welcome to participate in the call.  The link to the call will be posted on the mailing
list each week along with set topics for discussion.
Minutes for the call will be posted to the mailing list for those who are unable to join the call.

## Markdown style

To keep consistency throughout the Markdown files in the Open Container spec all files should be formatted one sentence per line.
This fixes two things: it makes diffing easier with git and it resolves fights about line wrapping length.
For example, this paragraph will span three lines in the Markdown source.

### Sign your work

The sign-off is a simple line at the end of the explanation for the
patch, which certifies that you wrote it or otherwise have the right to
pass it on as an open-source patch.  The rules are pretty simple: if you
can certify the below (from
[developercertificate.org](http://developercertificate.org/)):

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
