# Proposal: Specify user/ownership for COPY


This is a docs-first proposal based on a discussion on #docker-maintainers
([transcript](https://botbot.me/freenode/docker-maintainers/2015-01-02/?msg=28645971&page=1) 
up to [here](https://botbot.me/freenode/docker-maintainers/2015-01-02/?msg=28657537&page=4)).

## Background

The Dockerfile `COPY` instruction currently doesn't respect the user that is set
using `USER` instruction; files and directories copied to the container are 
always added with UID and GID 0. Workarounds, for example, doing `RUN chown ...`
don't work in all situations (for example when using `aufs`).

A previous PR, [#9046](https://github.com/docker/docker/pull/9046), solved this
problem by making `COPY` respect the user set via `USER`. Unfortunately, this
would be a backwards incompatible change and was closed because of this.

The issue leading to that PR can be found here [#7537](https://github.com/docker/docker/issues/7537).


## Primary goals

- Enable setting ownership of files and directories copied to containers when
  using the `COPY` instruction.
- Maintain backward compatibility with existing Dockerfiles


## Proposed change

Add a "configuration" option to the `COPY` instruction;

    user=<uid_or_username> COPY <src>... <dest>

All files and directories copied to the container will be owned by the
specified `<uid_or_username>`. After completing the `COPY`, the user that was
active *before* the instruction will be used for subsequent instructions.

The configuration is *optional*. If no configuration is provided, files and
directories are added with UID and GID of 0.

> **Note**:
> The discussion on #docker-maintainer does not mention specifying a group/GID.


## Future plans

Although not *explicitly* mentioned in the discussion, "configuration"
parameters can also be applied to other instructions in the future, for example,
to set the user for a `RUN` instruction. Other "configuration" settings can be
added as well.

This proposal is *primarily* for specifying the "user" for the `COPY` instruction. Still, future plans could be taken into consideration when
commenting.

## Known objections

A number of concerns were raised in response to the discussion on #docker-maintainers. I list them here, so that they don't have to be repeated
in this discussion.

- The `USER` instruction is still available and may confuse users (when to use
  `USER`, when to use `user=<uid_or_username>`?).
- The proposed syntax makes the `Dockerfile` harder to read; instructions no
  longer at the start of each line.
- Handling of white-space if more "configuration" options are added in
  the future.

## Process

To keep the discussion focused;

- The description of this proposal is also included as part of the PR. To
  comment on the description, create inline comments, this makes it easier to
  discuss things. The description on GitHub will be updated if commits are
  added.
- Please focus on the global "design" first. Comments on typos or
  language changes are welcome, but best kept for a later stage.
- I will update this PR by adding new commits to make tracking changes easier.
- If this proposal is accepted, I will hand the PR over to a maintainer for
  actual *implementation*. I'm not a Gopher :smile:
