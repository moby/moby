# Contributing to Docker

Want to hack on Docker? Awesome! Here are instructions to get you
started. They are probably not perfect, please let us know if anything
feels wrong or incomplete.

## Reporting Issues

When reporting [issues](https://github.com/dotcloud/docker/issues) 
on Github please include your host OS ( Ubuntu 12.04, Fedora 19, etc... )
and the output of `docker version` along with the output of `docker info` if possible.  
This information will help us review and fix your issue faster.

## Build Environment

For instructions on setting up your development environment, please
see our dedicated [dev environment setup
docs](http://docs.docker.io/en/latest/contributing/devenvironment/).

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
that feature *on top of* docker.

### Discuss your design on the mailing list

We recommend discussing your plans [on the mailing
list](https://groups.google.com/forum/?fromgroups#!forum/docker-dev)
before starting to code - especially for more ambitious contributions.
This gives other contributors a chance to point you in the right
direction, give feedback on your design, and maybe point out if someone
else is working on the same thing.

### Create issues...

Any significant improvement should be documented as [a github
issue](https://github.com/dotcloud/docker/issues) before anybody
starts working on it.

### ...but check for existing issues first!

Please take a moment to check that an issue doesn't already exist
documenting your bug report or improvement proposal. If it does, it
never hurts to add a quick "+1" or "I have this problem too". This will
help prioritize the most common problems and requests.

### Conventions

Fork the repo and make changes on your fork in a feature branch:

- If it's a bugfix branch, name it XXX-something where XXX is the number of the
  issue
- If it's a feature branch, create an enhancement issue to announce your
  intentions, and name it XXX-something where XXX is the number of the issue.

Submit unit tests for your changes.  Go has a great test framework built in; use
it! Take a look at existing tests for inspiration. Run the full test suite on
your branch before submitting a pull request.

Update the documentation when creating or modifying features. Test
your documentation changes for clarity, concision, and correctness, as
well as a clean documentation build. See ``docs/README.md`` for more
information on building the docs and how docs get released.

Write clean code. Universally formatted code promotes ease of writing, reading,
and maintenance. Always run `go fmt` before committing your changes. Most
editors have plugins that do this automatically, and there's also a git
pre-commit hook:

```
curl -o .git/hooks/pre-commit https://raw.github.com/edsrzf/gofmt-git-hook/master/fmt-check && chmod +x .git/hooks/pre-commit
```

Pull requests descriptions should be as clear as possible and include a
reference to all the issues that they address.

Code review comments may be added to your pull request. Discuss, then make the
suggested modifications and push additional commits to your feature branch. Be
sure to post a comment after pushing. The new commits will show up in the pull
request automatically, but the reviewers will not be notified unless you
comment.

Before the pull request is merged, make sure that you squash your commits into
logical units of work using `git rebase -i` and `git push -f`. After every
commit the test suite should be passing. Include documentation changes in the
same commit so that a revert would remove all traces of the feature or fix.

Commits that fix or close an issue should include a reference like `Closes #XXX`
or `Fixes #XXX`, which will automatically close the issue when merged.

Add your name to the AUTHORS file, but make sure the list is sorted and your
name and email address match your git configuration. The AUTHORS file is
regenerated occasionally from the git commit history, so a mismatch may result
in your changes being overwritten.

### Sign your work

The sign-off is a simple line at the end of the explanation for the
patch, which certifies that you wrote it or otherwise have the right to
pass it on as an open-source patch.  The rules are pretty simple: if you
can certify the below:

```
Docker Developer Grant and Certificate of Origin 1.0

By making a contribution to the Docker Project ("Project"), I represent and warrant that:

a. The contribution was created in whole or in part by me and I have the right to submit the contribution on my own behalf or on behalf of a third party who has authorized me to submit this contribution to the Project; or


b. The contribution is based upon previous work that, to the best of my knowledge, is covered under an appropriate open source license and I have the right and authorization to submit that work with modifications, whether created in whole or in part by me, under the same open source license (unless I am permitted to submit under a different license) that I have identified in the contribution; or

c. The contribution was provided directly to me by some other person who represented and warranted (a) or (b) and I have not modified it.

d. I understand and agree that this Project and the contribution are publicly known and that a record of the contribution (including all personal information I submit with it, including my sign-off record) is maintained indefinitely and may be redistributed consistent with this Project or the open source license(s) involved.

e. I hereby grant to the Project, dotCloud, Inc and its successors;  and recipients of software distributed by the Project a perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable copyright license to reproduce, modify, prepare derivative works of, publicly display, publicly perform, sublicense, and distribute this contribution and such modifications and derivative works consistent with this Project, the open source license indicated in the previous work or other appropriate open source license specified by the Project and approved by the Open Source Initiative(OSI) at http://www.opensource.org.
```

then you just add a line saying

    Docker-DCO-1.0-Signed-off-by: Joe Smith <joe.smith@email.com> @github_handle

using your real name (sorry, no pseudonyms or anonymous contributions.)

If you have any questions, please refer to the FAQ in the [docs](http://docs.docker.io)



### How can I become a maintainer?

* Step 1: learn the component inside out
* Step 2: make yourself useful by contributing code, bugfixes, support etc.
* Step 3: volunteer on the irc channel (#docker@freenode)

Don't forget: being a maintainer is a time investment. Make sure you will have time to make yourself available.
You don't have to be a maintainer to make a difference on the project!

