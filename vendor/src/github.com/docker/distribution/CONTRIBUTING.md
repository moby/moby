# Contributing to the registry

## Before reporting an issue...

### If your problem is with...

 - automated builds
 - your account on the [Docker Hub](https://hub.docker.com/)
 - any other [Docker Hub](https://hub.docker.com/) issue

Then please do not report your issue here - you should instead report it to [https://support.docker.com](https://support.docker.com)

### If you...

 - need help setting up your registry
 - can't figure out something
 - are not sure what's going on or what your problem is

Then please do not open an issue here yet - you should first try one of the following support forums:

 - irc: #docker-distribution on freenode
 - mailing-list: <distribution@dockerproject.org> or https://groups.google.com/a/dockerproject.org/forum/#!forum/distribution

## Reporting an issue properly

By following these simple rules you will get better and faster feedback on your issue.

 - search the bugtracker for an already reported issue

### If you found an issue that describes your problem:

 - please read other user comments first, and confirm this is the same issue: a given error condition might be indicative of different problems - you may also find a workaround in the comments
 - please refrain from adding "same thing here" or "+1" comments
 - you don't need to comment on an issue to get notified of updates: just hit the "subscribe" button
 - comment if you have some new, technical and relevant information to add to the case

### If you have not found an existing issue that describes your problem:

 1. create a new issue, with a succinct title that describes your issue:
   - bad title: "It doesn't work with my docker"
   - good title: "Private registry push fail: 400 error with E_INVALID_DIGEST"
 2. copy the output of:
   - `docker version`
   - `docker info`
   - `docker exec <registry-container> registry -version`
 3. copy the command line you used to launch your Registry
 4. restart your docker daemon in debug mode (add `-D` to the daemon launch arguments)
 5. reproduce your problem and get your docker daemon logs showing the error
 6. if relevant, copy your registry logs that show the error
 7. provide any relevant detail about your specific Registry configuration (e.g., storage backend used)
 8. indicate if you are using an enterprise proxy, Nginx, or anything else between you and your Registry

## Contributing a patch for a known bug, or a small correction

You should follow the basic GitHub workflow:

 1. fork
 2. commit a change
 3. make sure the tests pass
 4. PR

Additionally, you must [sign your commits](https://github.com/docker/docker/blob/master/CONTRIBUTING.md#sign-your-work). It's very simple:

 - configure your name with git: `git config user.name "Real Name" && git config user.email mail@example.com`
 - sign your commits using `-s`: `git commit -s -m "My commit"`

Some simple rules to ensure quick merge:

 - clearly point to the issue(s) you want to fix in your PR comment (e.g., `closes #12345`)
 - prefer multiple (smaller) PRs addressing individual issues over a big one trying to address multiple issues at once
 - if you need to amend your PR following comments, please squash instead of adding more commits

## Contributing new features

You are heavily encouraged to first discuss what you want to do. You can do so on the irc channel, or by opening an issue that clearly describes the use case you want to fulfill, or the problem you are trying to solve.

If this is a major new feature, you should then submit a proposal that describes your technical solution and reasoning.
If you did discuss it first, this will likely be greenlighted very fast. It's advisable to address all feedback on this proposal before starting actual work. 

Then you should submit your implementation, clearly linking to the issue (and possible proposal).

Your PR will be reviewed by the community, then ultimately by the project maintainers, before being merged.

It's mandatory to:

 - interact respectfully with other community members and maintainers - more generally, you are expected to abide by the [Docker community rules](https://github.com/docker/docker/blob/master/CONTRIBUTING.md#docker-community-guidelines)
 - address maintainers' comments and modify your submission accordingly
 - write tests for any new code

Complying to these simple rules will greatly accelerate the review process, and will ensure you have a pleasant experience in contributing code to the Registry.

Have a look at a great, succesful contribution: the [Ceph driver PR](https://github.com/docker/distribution/pull/443)

## Coding Style

Unless explicitly stated, we follow all coding guidelines from the Go
community. While some of these standards may seem arbitrary, they somehow seem
to result in a solid, consistent codebase.

The rules:

1. All code should be formatted with `gofmt -s`.
2. All code should pass the default levels of
   [`golint`](https://github.com/golang/lint).
3. All code should follow the guidelines covered at
   https://github.com/golang/go/wiki/CodeReviewComments.
4. Comment the code. Tell us the why, the history and the context.
5. Document _all_ declarations and methods, even private ones. Declare
   expectations, caveats and anything else that may be important. If a type
   gets exported, having the comments already there will ensure it's ready.
6. Variable name length should be proportional to it's context and no longer.
   noALongVariableNameLikeThisIsNotMoreClearWhenASimpleCommentWouldDo. In
   practice, short methods will have short variable names and globals will
   have longer names.
7. No underscores in package names. If you need a compound name, step back,
   and re-examine why you need a compound name. If you still think you need a
   compound name, lose the underscore.
8. No utils or helpers packages. If a function is not general enough to
   warrant it's own package, it has not been written generally enough to be a
   part of a util package. Just leave it unexported and well-documented.
9. No, we don't need another unit testing framework.
10. Even though we call these "rules" above, they are actually just
    guidelines. Since you've read all the rules, you now know that.

If you are having trouble getting into the mood of idiomatic Go, we recommend
reading through [`Effective Go`](http://golang.org/doc/effective_go.html). The
[Go Blog](http://blog.golang.org/) is also a great resource. Drinking the
kool-aid is a lot easier than going thirsty.
