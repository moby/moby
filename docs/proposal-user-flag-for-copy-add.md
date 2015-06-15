# Proposal: Implement --user and --group flags for COPY and ADD

This is a docs-first proposal for the implementation portion of the proposal
approved in #9934.


## Background

Numerous Issues have stressed the problem of allowing `USER` to be set, but
then files added via `COPY` or `ADD` by default are set to a UID and GID of 0,
often causing unexpected problems. A framework was implemented via PR #10775 to
allow user options/flags to be added to Dockerfile commands. This proposal aims
to add two flags for the `COPY` and `ADD` commands to allow the user to specify
the ownership of the files/directories copied/added using those commands.
Additional discussion and approval was handled via proposal #9934.


## Primary goals

- Implement the use of the `--user=<uid_or_username>` and 
  `--group=<gid_or_groupname>` flags for `COPY` and `ADD`, leveraging the 
  framework implemented in PR #10775.
- These flags will allow the user to specify the ownership permissions for the
  files copied/added using the `COPY` and `ADD` commands.
- Maintain backwards compatibility (flag is optional, will continue the default
  behavior if not specified)
- Update documentation to reflect this new implementation.


## Proposed change

Add configuration flag to `COPY` and `ADD`:

    COPY --user=<uid_or_username> <src>... <dest>
    ADD --user=<uid_or_username> <src> ... <dest>

    COPY --group=<gid_or_groupname> <src>... <dest>
    ADD --group=<gid_or_groupname> <src> ... <dest>

    COPY --user=<uid_or_username> --group=<gid_or_groupname> <src>... <dest>
    ADD --user=<uid_or_username> --group=<gid_or_groupname> <src> ... <dest>

All files and directories copied to the container will be owned by the 
specified <uid_or_username> and/or <gid_or_groupname>. Use of the `--user` and
`--group` flags will only affect the current `COPY` or `ADD` command.
 
The configuration is optional. If the `--user` and/or `--group` flag are not
provided, files and directories are added in the default manner with UID and
GID of 0. (A check will be performed for the flag, if not present the
original/default manner is followed).
  
`docker/libcontainer` has existing code for providing the permission bits for a
given user/group. This existing function will be leveraged to determine the
correct permission bits to set on the newly copied file within the container.

## Implementation

1. Create Dockerfile COPY/ADD flags (`--user=<uid_or_username>` and `--group=<gid_or_groupname>`)
in builder/dispatchers.go to be used for both `COPY` and `ADD` commands
(`func dispatchCopy` and `func add`)
2. Use CMD framework from PR #10775 to parse for the `--user=<uid_or_username>`
and/or `--group=<gid_or_groupname>` in both `COPY` and `ADD` commands
3. Pass the specified user and/or group as additional parameters to
`runContextCommand(...)`, or `nil` if not present
4. In builder/internals.go update `func runContextCommand` to take the
additional variables
5. In builder/internals.go update `func addContext` to take and check for the 
user and group variable
 - if `nil` continue in default manner (set Uid and Gid to 0)
 - else call `func GetExecUserPath` from libcontainer/user/user.go to return 
   the `ExecUser`
 - if user does not exist throw error, and do not continue on to Step 6
6. If the `ExecUser` is returned, pass the Uid and Gid to `copyAsDirectory(...)`,
`fixFilePermissions(...)`, etc., else if continuing in the default manner, 
pass 0 for Uid and Gid.


## Known objections

- Must be backwards compatible (that is why the flag is optional)
- Should not be tied to the USER command


## Process

- The description of this proposal is also included as part of the PR. To
  comment on the description, create inline comments, this makes it easier to 
  discuss things.
- Please focus on the `--user` and `--group` flag implementation since this
  general idea has been discussed and approved via PR #9934.
