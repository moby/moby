# The Docker maintainer manual

## Introduction

Dear maintainer. Thank you for investing the time and energy to help make Docker as
useful as possible. Maintaining a project is difficult, sometimes unrewarding work.
Sure, you will get to contribute cool features to the project. But most of your time
will be spent reviewing, cleaning up, documenting, andswering questions, justifying
design decisions - while everyone has all the fun! But remember - the quality of the
maintainers work is what distinguishes the good projects from the great.
So please be proud of your work, even the unglamourous parts, and encourage a culture
of appreciation and respect for *every* aspect of improving the project - not just the
hot new features.

This document is a manual for maintainers old and new. It explains what is expected of
maintainers, how they should work, and what tools are available to them.

This is a living document - if you see something out of date or missing, speak up!


## What are a maintainer's responsibility?

It is every maintainer's responsibility to:

* 1) Expose a clear roadmap for improving their component.
* 2) Deliver prompt feedback and decisions on pull requests.
* 3) Be available to anyone with questions, bug reports, criticism etc. on their component. This includes irc, github requests and the mailing list.
* 4) Make sure their component respects the philosophy, design and roadmap of the project.


## How are decisions made?

Short answer: with pull requests to the docker repository.

Docker is an open-source project with an open design philosophy. This means that the repository is the source of truth for EVERY aspect of the project,
including its philosophy, design, roadmap and APIs. *If it's part of the project, it's in the repo. It's in the repo, it's part of the project.*

As a result, all decisions can be expressed as changes to the repository. An implementation change is a change to the source code. An API change is a change to
the API specification. A philosophy change is a change to the philosophy manifesto. And so on.

All decisions affecting docker, big and small, follow the same 3 steps:

* Step 1: Open a pull request. Anyone can do this.

* Step 2: Discuss the pull request. Anyone can do this.

* Step 3: Accept or refuse a pull request. The relevant maintainer does this (see below "Who decides what?")


## Who decides what?

So all decisions are pull requests, and the relevant maintainer makes the decision by accepting or refusing the pull request.
But how do we identify the relevant maintainer for a given pull request?

Docker follows the timeless, highly efficient and totally unfair system known as [Benevolent dictator for life](http://en.wikipedia.org/wiki/Benevolent_Dictator_for_Life),
with yours truly, Solomon Hykes, in the role of BDFL.
This means that all decisions are made by default by me. Since making every decision myself would be highly unscalable, in practice decisions are spread across multiple maintainers.

The relevant maintainer for a pull request is assigned in 3 steps:

* Step 1: Determine the subdirectory affected by the pull request. This might be src/registry, docs/source/api, or any other part of the repo.

* Step 2: Find the MAINTAINERS file which affects this directory. If the directory itself does not have a MAINTAINERS file, work your way up the the repo hierarchy until you find one.

* Step 3: The first maintainer listed is the primary maintainer. The pull request is assigned to him. He may assign it to other listed maintainers, at his discretion.


### I'm a maintainer, should I make pull requests too?

Primary maintainers are not required to create pull requests when changing their own subdirectory, but secondary maintainers are.

### Who assigns maintainers?

Solomon.

### How is this process changed?

Just like everything else: by making a pull request :)
