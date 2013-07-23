# Docker maintainer bootcamp

## Introduction: we need more maintainers

Docker is growing incredibly fast. At the time of writing, it has received over 200 contributions from 90 people,
and its API is used by dozens of 3d-party tools. Over 1,000 issues have been opened. As the first production deployments
start going live, the growth will only accelerate.

Also at the time of writing, Docker has 3 full-time maintainers, and 7 part-time subsystem maintainers. If docker
is going to live up to the expectations, we need more than that.

This document describes a *bootcamp* to guide and train volunteers interested in helping the project, either with individual
contributions, maintainer work, or both.

This bootcamp is an experiment. If you decide to go through it, consider yourself an alpha-tester. You should expect quirks,
and report them to us as you encounter them to help us smooth out the process.


## How it works

The maintainer bootcamp is a 12-step program - one step for each of the maintainer's responsibilities. The aspiring maintainer must
validate all 12 steps by 1) studying it, 2) practicing it, and 3) getting endorsed for it.

Steps are all equally important and can be validated in any order. Validating all 12 steps is a pre-requisite for becoming a core
maintainer, but even 1 step will make you a better contributor!

### List of steps

#### 1) Be a power user

Use docker daily, build cool things with it, know its quirks inside and out.


#### 2) Help users

Answer questions on irc, twitter, email, in person.


#### 3) Manage the bug tracker

Help triage tickets - ask the right questions, find duplicates, reference relevant resources, know when to close a ticket when necessary, take the time to go over older tickets.


#### 4) Improve the documentation

Follow the documentation from scratch regularly and make sure it is still up-to-date. Find and fix inconsistencies. Remove stale information. Find a frequently asked question that is not documented. Simplify the content and the form.


#### 5) Evangelize the principles of docker

Understand what the underlying goals and principle of docker are. Explain design decisions based on what docker is, and what it is not. When someone is not using docker, find how docker can be valuable to them. If they are using docker, find how they can use it better.


#### 6) Fix bugs

Self-explanatory. Contribute improvements to docker which solve defects. Bugfixes should be well-tested, and prioritized by impact to the user.


#### 7) Improve the testing infrastructure

Automated testing is complicated and should be perpetually improved. Invest time to improve the current tooling. Refactor existing tests, create new ones, make testing more accessible to developers, add new testing capabilities (integration tests, mocking, stress test...), improve integration between tests and documentation...


#### 8) Contribute features

Improve docker to do more things, or get better at doing the same things. Features should be well-tested, not break existing APIs, respect the project goals. They should make the user's life measurably better. Features should be discussed ahead of time to avoid wasting time and duplicating effort.


#### 9) Refactor internals

Improve docker to repay technical debt. Simplify code layout, improve performance, add missing comments, reduce the number of files and functions, rename functions and variables to be more readable, go over FIXMEs, etc.

#### 10) Review and merge contributions

Review pull requests in a timely manner, review code in detail and offer feedback. Keep a high bar without being pedantic. Share the load of testing and merging pull requests.

#### 11) Release

Manage a release of docker from beginning to end. Tests, final review, tags, builds, upload to mirrors, distro packaging, etc.

#### 12) Train other maintainers

Contribute to training other maintainers. Give advic


### How to study a step

### How to practice a step

### How to get endorsed for a step


