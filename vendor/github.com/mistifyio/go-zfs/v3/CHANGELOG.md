# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).
This change log follows the advice of [Keep a CHANGELOG](https://github.com/olivierlacan/keep-a-changelog).

## [Unreleased]

## [3.0.0] - 2022-03-30

### Added

- Rename, Mount and Unmount methods
- Parse more fields into Zpool type:
  - dedupratio
  - fragmentation
  - freeing
  - leaked
  - readonly
- Parse more fields into Dataset type:
  - referenced
- Incremental Send
- Parse numbers in exact format
- Support for Solaris (non-blockint, best-effort status)
- Debug logging for command invocation
- Use GitHub Actions for CI
- Nix shell for dev env reproducibility
- Direnv file for ease of dev
- Formatting/lint checks (enforced by CI)
- Go Module
- FreeBSD based vagrant machine

### Changed

- Temporarily adjust TestDiff expected strings depending on ZFS version
- Use one `zfs list`/`zpool list` call instead of many `zfs get`/`zpool get`
- ZFS docs links now point to OpenZFS pages
- Ubuntu vagrant box changed to generic/ubuntu2004

### Fixed

- `GetProperty` returning `VALUE` instead of the actual value

### Shortlog

    Amit Krishnan (1):
      Issue #39 and Issue #40 - Enable Solaris support for go-zfs Switch from zfs/zpool get to zfs/zpool list for better performance Signed-off-by: Amit Krishnan <krish.amit@gmail.com>

    Anand Patil (3):
      Added Rename
      Small fix to rename.
      Added mount and umount methods

    Brian Akins (1):
      Add 'referenced' to zfs properties

    Brian Bickerton (3):
      Add debug logging before and after running external zfs command
      Don't export the default no-op logger
      Update uuid package repo url

    Dmitry Teselkin (1):
      Issue #52 - fix parseLine for fragmentation field

    Edward Betts (1):
      correct spelling mistake

    Justin Cormack (1):
      Switch to google/uuid which is the maintained version of pborman/uuid

    Manuel Mendez (40):
      rename Umount -> Unmount to follow zfs command name
      add missing Unmount/Mount docs
      always allocate largest Mount slice
      add travis config
      travis: update to go 1.7
      travis: get go deps first
      test: add nok helper to verify an error occurred
      test: add test for Dataset.GetProperty
      ci: swap #cerana on freenode for slack
      ci: install new deps for 0.7 relases
      ci: bump zol versions
      ci: bump go versions
      ci: use better gometalinter invocations
      ci: add ccache
      ci: set env earlier in before_install
      fix test nok error printing
      test: restructure TestDiff to deal with different order of changes
      test: better unicode path handling in TestDiff
      travis: bump zfs and go versions
      cache zfs artifacts
      Add nix-shell and direnv goodness
      prettierify all the files
      Add go based tools
      Add Makefile and rules.mk files
      gofumptize the code base
      Use tinkerbell/lint-install to setup linters
      make golangci-lint happy
      Update CONTRIBUTING.md with make based approach
      Add GitHub Actions
      Drop Travis CI
      One sentence per line
      Update documentation links to openzfs-docs pages
      Format Vagrantfile using rufo
      Add go-zfs.test to .gitignore
      test: Avoid reptitive/duplicate error logging and quitting
      test: Use t.Logf instead of fmt.Printf
      test: Better cleanup and error handling in zpoolTest
      test: Do not mark TestDatasets as a t.Helper.
      test: Change zpoolTest to a pure helper that returns a clean up function
      test: Move helpers to a different file
      vagrant: Add set -euxo pipefail to provision script
      vagrant: Update to generic/ubuntu2004
      vagrant: Minor fixes to Vagrantfile
      vagrant: Update to go 1.17.8
      vagrant: Run go tests as part of provision script
      vagrant: Indent heredoc script
      vagrant: Add freebsd machine

    Matt Layher (1):
      Parse more fields into Zpool type

    Michael Crosby (1):
      Add incremental send

    Rikard Gynnerstedt (1):
      remove command name from joined args

    Sebastiaan van Stijn (1):
      Add go.mod and rename to github.com/mistifyio/go-zfs/v3 (v3.0.0)

    mikudeko (1):
      Fix GetProperty always returning 'VALUE'

## [2.1.1] - 2015-05-29

### Fixed

- Ignoring first pool listed
- Incorrect `zfs get` argument ordering

### Shortlog

    Alexey Guskov (1):
      zfs command uses different order of arguments on freebsd

    Brian Akins (4):
      test that ListZpools returns expected zpool
      test error first
      test error first
      fix test to check correct return value

    James Cunningham (1):
      Fix Truncating First Zpool

    Pat Norton (2):
      Added Use of Go Tools
      Update CONTRIBUTING.md

## [2.1.0] - 2014-12-08

### Added

- Parse hardlink modification count returned from `zfs diff`

### Fixed

- Continuing instead of erroring when rolling back a non-snapshot

### Shortlog

    Brian Akins (2):
      need to return the error here
      use named struct fields

    Jörg Thalheim (1):
      zfs diff handle hardlinks modification now

## [2.0.0] - 2014-12-02

### Added

- Flags for Destroy:
  - DESTROY_DEFAULT
  - DESTROY_DEFER_DELETION (`zfs destroy ... -d`)
  - DESTROY_FORCE (`zfs destroy ... -f`)
  - DESTROY_RECURSIVE_CLONES (`zfs destroy ... -R`)
  - DESTROY_RECURSIVE (`zfs destroy ... -r`)
  - etc
- Diff method (`zfs diff`)
- LogicalUsed and Origin properties to Dataset
- Type constants for Dataset
- State constants for Zpool
- Logger interface
- Improve documentation

### Shortlog

    Brian Akins (8):
      remove reflection
      style change for switches
      need to check for error
      keep in scope
      go 1.3.3
      golint cleanup
      Just test if logical used is greater than 0, as this appears to be implementation specific
      add docs to satisfy golint

    Jörg Thalheim (8):
      Add deferred flag to zfs.Destroy()
      add Logicalused property
      Add Origin property
      gofmt
      Add zfs.Diff
      Add Logger
      add recursive destroy with clones
      use CamelCase-style constants

    Matt Layher (4):
      Improve documentation, document common ZFS operations, provide more references
      Add zpool state constants, for easier health checking
      Add dataset type constants, for easier type checking
      Fix string split in command.Run(), use strings.Fields() instead of strings.Split()

## [1.0.0] - 2014-11-12

### Shortlog

    Brian Akins (7):
      add godoc badge
      Add example
      add information about zpool to struct and parser
      Add Quota
      add Children call
      add Children call
      fix snapshot tests

    Brian Bickerton (3):
      MIST-150 Change Snapshot second paramater from properties map[string][string] to recursive bool
      MIST-150 Add Rollback method and related tests
      MIST-160 Add SendSnapshot streaming method and tests

    Matt Layher (1):
      Add Error struct type and tests, enabling easier error return checking

[3.0.0]: https://github.com/mistifyio/go-zfs/compare/v2.1.1...v3.0.0
[2.1.1]: https://github.com/mistifyio/go-zfs/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/mistifyio/go-zfs/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/mistifyio/go-zfs/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/mistifyio/go-zfs/compare/v0.0.0...v1.0.0
