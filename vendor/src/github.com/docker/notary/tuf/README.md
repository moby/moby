# GOTUF 

This is still a work in progress but will shortly be a fully compliant 
Go implementation of [The Update Framework (TUF)](http://theupdateframework.com/).

## Where's the CLI

This repository provides a library only. The [Notary project](https://github.com/docker/notary)
from Docker should be considered the official CLI to be used with this implementation of TUF.

## TODOs:

- [X] Add Targets to existing repo
- [X] Sign metadata files
- [X] Refactor TufRepo to take care of signing ~~and verification~~
- [ ] Ensure consistent capitalization in naming (TUF\_\_\_ vs Tuf\_\_\_)
- [X] Make caching of metadata files smarter - PR #5
- [ ] ~~Add configuration for CLI commands. Order of configuration priority from most to least: flags, config file, defaults~~ Notary should be the official CLI
- [X] Reasses organization of data types. Possibly consolidate a few things into the data package but break up package into a few more distinct files
- [ ] Comprehensive test cases
- [ ] Delete files no longer in use
- [ ] Fix up errors. Some have to be instantiated, others don't, the inconsistency is annoying.
- [X] Bump version numbers in meta files (could probably be done better)

## Credits

This implementation was originally forked from [flynn/go-tuf](https://github.com/flynn/go-tuf),
however in attempting to add delegations I found I was making such
significant changes that I could not maintain backwards compatibility
without the code becoming overly convoluted.

Some features such as pluggable verifiers have already been merged upstream to flynn/go-tuf
and we are in discussion with [titanous](https://github.com/titanous) about working to merge the 2 implementations.

This implementation retains the same 3 Clause BSD license present on 
the original flynn implementation.
