# Contributing to Docker

Want to hack on Docker? Awesome! Here are instructions to get you
started. They are probably not perfect, please let us know if anything
feels wrong or incomplete.

## Topics

* [Security Reports](#security-reports)
* [Design and Cleanup Proposals](#design-and-cleanup-proposals)
* [Reporting Issues](#reporting-issues)
* [Build Environment](#build-environment)
* [Contribution Guidelines](#contribution-guidelines)
* [Community Guidelines](#docker-community-guidelines)

## Security Reports

Please **DO NOT** file an issue for security related issues. Please send your
reports to [security@docker.com](mailto:security@docker.com) instead.

## Design and Cleanup Proposals

When considering a design proposal, we are looking for:

* A description of the problem this design proposal solves
* An issue -- not a pull request -- that describes what you will take action on
  * Please prefix your issue with `Proposal:` in the title
* Please review [the existing Proposals](https://github.com/dotcloud/docker/issues?direction=asc&labels=Proposal&page=1&sort=created&state=open)
  before reporting a new issue. You can always pair with someone if you both
  have the same idea.

When considering a cleanup task, we are looking for:

* A description of the refactors made
  * Please note any logic changes if necessary
* A pull request with the code
  * Please prefix your PR's title with `Cleanup:` so we can quickly address it.
  * Your pull request must remain up to date with master, so rebase as necessary.

## Reporting Issues

When reporting [issues](https://github.com/docker/docker/issues) on
GitHub please include your host OS (Ubuntu 12.04, Fedora 19, etc).
Please include:

* The output of `uname -a`.
* The output of `docker version`.
* The output of `docker -D info`.

Please also include the steps required to reproduce the problem if
possible and applicable.  This information will help us review and fix
your issue faster.

## Build Environment

For instructions on setting up your development environment, please
see our dedicated [dev environment setup
docs](http://docs.docker.com/contributing/devenvironment/).

## Contribution guidelines

### Pull requests are always welcome

We are always thrilled to receive pull requests, and do our best to
process them as fast as possible. Not sure if that typo is worth a pull
request? Do it! We will appreciate it.

If your pull request is not accepted on the first try, don't be
discouraged! If there's a problem with the implementation, hopefully you
received feedback on what to improve.

We're trying very hard to keep Docker lean and focused. We don't want it
to do everything for everybody. This means that we might decide against
incorporating a new feature. However, there might be a way to implement
that feature *on top of* Docker.

### Discuss your design on the mailing list

We recommend discussing your plans [on the mailing
list](https://groups.google.com/forum/?fromgroups#!forum/docker-dev)
before starting to code - especially for more ambitious contributions.
This gives other contributors a chance to point you in the right
direction, give feedback on your design, and maybe point out if someone
else is working on the same thing.

### Create issues...

Any significant improvement should be documented as [a GitHub
issue](https://github.com/docker/docker/issues) before anybody
starts working on it.

### ...but check for existing issues first!

Please take a moment to check that an issue doesn't already exist
documenting your bug report or improvement proposal. If it does, it
never hurts to add a quick "+1" or "I have this problem too". This will
help prioritize the most common problems and requests.

### Conventions

Fork the repository and make changes on your fork in a feature branch:

- If it's a bug fix branch, name it XXXX-something where XXXX is the number of the
  issue.
- If it's a feature branch, create an enhancement issue to announce your
  intentions, and name it XXXX-something where XXXX is the number of the issue.

Submit unit tests for your changes.  Go has a great test framework built in; use
it! Take a look at existing tests for inspiration. Run the full test suite on
your branch before submitting a pull request.

Update the documentation when creating or modifying features. Test
your documentation changes for clarity, concision, and correctness, as
well as a clean documentation build. See `docs/README.md` for more
information on building the docs and how they get released.

Write clean code. Universally formatted code promotes ease of writing, reading,
and maintenance. Always run `gofmt -s -w file.go` on each changed file before
committing your changes. Most editors have plug-ins that do this automatically.

Pull requests descriptions should be as clear as possible and include a
reference to all the issues that they address.

Commit messages must start with a capitalized and short summary (max. 50
chars) written in the imperative, followed by an optional, more detailed
explanatory text which is separated from the summary by an empty line.

Code review comments may be added to your pull request. Discuss, then make the
suggested modifications and push additional commits to your feature branch. Be
sure to post a comment after pushing. The new commits will show up in the pull
request automatically, but the reviewers will not be notified unless you
comment.

Pull requests must be cleanly rebased ontop of master without multiple branches
mixed into the PR.

**Git tip**: If your PR no longer merges cleanly, use `rebase master` in your
feature branch to update your pull request rather than `merge master`.

Before the pull request is merged, make sure that you squash your commits into
logical units of work using `git rebase -i` and `git push -f`. After every
commit the test suite should be passing. Include documentation changes in the
same commit so that a revert would remove all traces of the feature or fix.

Commits that fix or close an issue should include a reference like
`Closes #XXXX` or `Fixes #XXXX`, which will automatically close the
issue when merged.

Please do not add yourself to the `AUTHORS` file, as it is regenerated
regularly from the Git history.

### Merge approval

Docker maintainers use LGTM (Looks Good To Me) in comments on the code review
to indicate acceptance.

A change requires LGTMs from an absolute majority of the maintainers of each
component affected. For example, if a change affects `docs/` and `registry/`, it
needs an absolute majority from the maintainers of `docs/` AND, separately, an
absolute majority of the maintainers of `registry/`.

For more details see [MAINTAINERS.md](hack/MAINTAINERS.md)

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

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Using your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

Note that the old-style `Docker-DCO-1.1-Signed-off-by: ...` format is still
accepted, so there is no need to update outstanding pull requests to the new
format right away, but please do adjust your processes for future contributions.

#### Small patch exception

There are several exceptions to the signing requirement. Currently these are:

* Your patch fixes spelling or grammar errors.
* Your patch is a single line change to documentation contained in the
  `docs` directory.
* Your patch fixes Markdown formatting or syntax errors in the
  documentation contained in the `docs` directory.

If you have any questions, please refer to the FAQ in the [docs](http://docs.docker.com)

### How can I become a maintainer?

* Step 1: Learn the component inside out
* Step 2: Make yourself useful by contributing code, bug fixes, support etc.
* Step 3: Volunteer on the IRC channel (#docker at Freenode)
* Step 4: Propose yourself at a scheduled docker meeting in #docker-dev

Don't forget: being a maintainer is a time investment. Make sure you
will have time to make yourself available.  You don't have to be a
maintainer to make a difference on the project!

### IRC Meetings

There are two monthly meetings taking place on #docker-dev IRC to accomodate all timezones.
Anybody can ask for a topic to be discussed prior to the meeting.

If you feel the conversation is going off-topic, feel free to point it out.

For the exact dates and times, have a look at [the irc-minutes repo](https://github.com/docker/irc-minutes).
They also contain all the notes from previous meetings.

## Docker Community Guidelines

We want to keep the Docker community awesome, growing and collaborative. We
need your help to keep it that way. To help with this we've come up with some
general guidelines for the community as a whole:

* Be nice: Be courteous, respectful and polite to fellow community members: no
  regional, racial, gender, or other abuse will be tolerated. We like nice people
  way better than mean ones!

* Encourage diversity and participation: Make everyone in our community
  feel welcome, regardless of their background and the extent of their
  contributions, and do everything possible to encourage participation in
  our community.

* Keep it legal: Basically, don't get us in trouble. Share only content that
  you own, do not share private or sensitive information, and don't break the
  law.

* Stay on topic: Make sure that you are posting to the correct channel
  and avoid off-topic discussions. Remember when you update an issue or
  respond to an email you are potentially sending to a large number of
  people.  Please consider this before you update.  Also remember that
  nobody likes spam.

### Guideline Violations — 3 Strikes Method

The point of this section is not to find opportunities to punish people, but we
do need a fair way to deal with people who are making our community suck.

1. First occurrence: We'll give you a friendly, but public reminder that the
   behavior is inappropriate according to our guidelines.

2. Second occurrence: We will send you a private message with a warning that
   any additional violations will result in removal from the community.

3. Third occurrence: Depending on the violation, we may need to delete or ban
   your account.

**Notes:**

* Obvious spammers are banned on first occurrence. If we don't do this, we'll
  have spam all over the place.

* Violations are forgiven after 6 months of good behavior, and we won't
  hold a grudge.

* People who commit minor infractions will get some education,
  rather than hammering them in the 3 strikes process.

* The rules apply equally to everyone in the community, no matter how
  much you've contributed.

* Extreme violations of a threatening, abusive, destructive or illegal nature
  will be addressed immediately and are not subject to 3 strikes or
  forgiveness.

* Contact james@docker.com to report abuse or appeal violations. In the case of
  appeals, we know that mistakes happen, and we'll work with you to come up with
  a fair solution if there has been a misunderstanding.

