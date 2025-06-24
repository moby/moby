<a name="unreleased"></a>
## [Unreleased]


<a name="v0.7.1"></a>
## [v0.7.1] - 2023-05-11
### Add
- Add describe functions ([#77](https://github.com/montanaflynn/stats/issues/77))

### Update
- Update .gitignore
- Update README.md, LICENSE and DOCUMENTATION.md files
- Update github action go workflow to run on push


<a name="v0.7.0"></a>
## [v0.7.0] - 2023-01-08
### Add
- Add geometric distribution functions ([#75](https://github.com/montanaflynn/stats/issues/75))
- Add GitHub action go workflow

### Remove
- Remove travis CI config

### Update
- Update changelog with v0.7.0 changes
- Update changelog with v0.7.0 changes
- Update github action go workflow
- Update geometric distribution tests


<a name="v0.6.6"></a>
## [v0.6.6] - 2021-04-26
### Add
- Add support for string and io.Reader in LoadRawData (pr [#68](https://github.com/montanaflynn/stats/issues/68))
- Add latest versions of Go to test against

### Update
- Update changelog with v0.6.6 changes

### Use
- Use math.Sqrt in StandardDeviation (PR [#64](https://github.com/montanaflynn/stats/issues/64))


<a name="v0.6.5"></a>
## [v0.6.5] - 2021-02-21
### Add
- Add Float64Data.Quartiles documentation
- Add Quartiles method to Float64Data type (issue [#60](https://github.com/montanaflynn/stats/issues/60))

### Fix
- Fix make release changelog command and add changelog history

### Update
- Update changelog with v0.6.5 changes
- Update changelog with v0.6.4 changes
- Update README.md links to CHANGELOG.md and DOCUMENTATION.md
- Update README.md and Makefile with new release commands


<a name="v0.6.4"></a>
## [v0.6.4] - 2021-01-13
### Fix
- Fix failing tests due to precision errors on arm64 ([#58](https://github.com/montanaflynn/stats/issues/58))

### Update
- Update changelog with v0.6.4 changes
- Update examples directory to include a README.md used for synopsis
- Update go.mod to include go version where modules are enabled by default
- Update changelog with v0.6.3 changes


<a name="v0.6.3"></a>
## [v0.6.3] - 2020-02-18
### Add
- Add creating and committing changelog to Makefile release directive
- Add release-notes.txt and .chglog directory to .gitignore

### Update
- Update exported tests to use import for better example documentation
- Update documentation using godoc2md
- Update changelog with v0.6.2 release


<a name="v0.6.2"></a>
## [v0.6.2] - 2020-02-18
### Fix
- Fix linting errcheck warnings in go benchmarks

### Update
- Update Makefile release directive to use correct release name


<a name="v0.6.1"></a>
## [v0.6.1] - 2020-02-18
### Add
- Add StableSample function signature to readme

### Fix
- Fix linting warnings for normal distribution functions formatting and tests

### Update
- Update documentation links and rename DOC.md to DOCUMENTATION.md
- Update README with link to pkg.go.dev reference and release section
- Update Makefile with new changelog, docs, and release directives
- Update DOC.md links to GitHub source code
- Update doc.go comment and add DOC.md package reference file
- Update changelog using git-chglog


<a name="v0.6.0"></a>
## [v0.6.0] - 2020-02-17
### Add
- Add Normal Distribution Functions ([#56](https://github.com/montanaflynn/stats/issues/56))
- Add previous versions of Go to travis CI config
- Add check for distinct values in Mode function ([#51](https://github.com/montanaflynn/stats/issues/51))
- Add StableSample function ([#48](https://github.com/montanaflynn/stats/issues/48))
- Add doc.go file to show description and usage on godoc.org
- Add comments to new error and legacy error variables
- Add ExampleRound function to tests
- Add go.mod file for module support
- Add Sigmoid, SoftMax and Entropy methods and tests
- Add Entropy documentation, example and benchmarks
- Add Entropy function ([#44](https://github.com/montanaflynn/stats/issues/44))

### Fix
- Fix percentile when only one element ([#47](https://github.com/montanaflynn/stats/issues/47))
- Fix AutoCorrelation name in comments and remove unneeded Sprintf

### Improve
- Improve documentation section with command comments

### Remove
- Remove very old versions of Go in travis CI config
- Remove boolean comparison to get rid of gometalinter warning

### Update
- Update license dates
- Update Distance functions signatures to use Float64Data
- Update Sigmoid examples
- Update error names with backward compatibility

### Use
- Use relative link to examples/main.go
- Use a single var block for exported errors


<a name="v0.5.0"></a>
## [v0.5.0] - 2019-01-16
### Add
- Add Sigmoid and Softmax functions

### Fix
- Fix syntax highlighting and add CumulativeSum func


<a name="v0.4.0"></a>
## [v0.4.0] - 2019-01-14
### Add
- Add goreport badge and documentation section to README.md
- Add Examples to test files
- Add AutoCorrelation and nist tests
- Add String method to statsErr type
- Add Y coordinate error for ExponentialRegression
- Add syntax highlighting ([#43](https://github.com/montanaflynn/stats/issues/43))
- Add CumulativeSum ([#40](https://github.com/montanaflynn/stats/issues/40))
- Add more tests and rename distance files
- Add coverage and benchmarks to azure pipeline
- Add go tests to azure pipeline

### Change
- Change travis tip alias to master
- Change codecov to coveralls for code coverage

### Fix
- Fix a few lint warnings
- Fix example error

### Improve
- Improve test coverage of distance functions

### Only
- Only run travis on stable and tip versions
- Only check code coverage on tip

### Remove
- Remove azure CI pipeline
- Remove unnecessary type conversions

### Return
- Return EmptyInputErr instead of EmptyInput

### Set
- Set up CI with Azure Pipelines


<a name="0.3.0"></a>
## [0.3.0] - 2017-12-02
### Add
- Add Chebyshev, Manhattan, Euclidean and Minkowski distance functions ([#35](https://github.com/montanaflynn/stats/issues/35))
- Add function for computing chebyshev distance. ([#34](https://github.com/montanaflynn/stats/issues/34))
- Add support for time.Duration
- Add LoadRawData to docs and examples
- Add unit test for edge case that wasn't covered
- Add unit tests for edge cases that weren't covered
- Add pearson alias delegating to correlation
- Add CovariancePopulation to Float64Data
- Add pearson product-moment correlation coefficient
- Add population covariance
- Add random slice benchmarks
- Add all applicable functions as methods to Float64Data type
- Add MIT license badge
- Add link to examples/methods.go
- Add Protips for usage and documentation sections
- Add tests for rounding up
- Add webdoc target and remove linting from test target
- Add example usage and consolidate contributing information

### Added
- Added MedianAbsoluteDeviation

### Annotation
- Annotation spelling error

### Auto
- auto commit
- auto commit

### Calculate
- Calculate correlation with sdev and covp

### Clean
- Clean up README.md and add info for offline docs

### Consolidated
- Consolidated all error values.

### Fix
- Fix Percentile logic
- Fix InterQuartileRange method test
- Fix zero percent bug and add test
- Fix usage example output typos

### Improve
- Improve bounds checking in Percentile
- Improve error log messaging

### Imput
- Imput -> Input

### Include
- Include alternative way to set Float64Data in example

### Make
- Make various changes to README.md

### Merge
- Merge branch 'master' of github.com:montanaflynn/stats
- Merge master

### Mode
- Mode calculation fix and tests

### Realized
- Realized the obvious efficiency gains of ignoring the unique numbers at the beginning of the slice.  Benchmark joy ensued.

### Refactor
- Refactor testing of Round()
- Refactor setting Coordinate y field using Exp in place of Pow
- Refactor Makefile and add docs target

### Remove
- Remove deep links to types and functions

### Rename
- Rename file from types to data

### Retrieve
- Retrieve InterQuartileRange for the Float64Data.

### Split
- Split up stats.go into separate files

### Support
- Support more types on LoadRawData() ([#36](https://github.com/montanaflynn/stats/issues/36))

### Switch
- Switch default and check targets

### Update
- Update Readme
- Update example methods and some text
- Update README and include Float64Data type method examples

### Pull Requests
- Merge pull request [#32](https://github.com/montanaflynn/stats/issues/32) from a-robinson/percentile
- Merge pull request [#30](https://github.com/montanaflynn/stats/issues/30) from montanaflynn/fix-test
- Merge pull request [#29](https://github.com/montanaflynn/stats/issues/29) from edupsousa/master
- Merge pull request [#27](https://github.com/montanaflynn/stats/issues/27) from andrey-yantsen/fix-percentile-out-of-bounds
- Merge pull request [#25](https://github.com/montanaflynn/stats/issues/25) from kazhuravlev/patch-1
- Merge pull request [#22](https://github.com/montanaflynn/stats/issues/22) from JanBerktold/time-duration
- Merge pull request [#24](https://github.com/montanaflynn/stats/issues/24) from alouche/master
- Merge pull request [#21](https://github.com/montanaflynn/stats/issues/21) from brydavis/master
- Merge pull request [#19](https://github.com/montanaflynn/stats/issues/19) from ginodeis/mode-bug
- Merge pull request [#17](https://github.com/montanaflynn/stats/issues/17) from Kunde21/master
- Merge pull request [#3](https://github.com/montanaflynn/stats/issues/3) from montanaflynn/master
- Merge pull request [#2](https://github.com/montanaflynn/stats/issues/2) from montanaflynn/master
- Merge pull request [#13](https://github.com/montanaflynn/stats/issues/13) from toashd/pearson
- Merge pull request [#12](https://github.com/montanaflynn/stats/issues/12) from alixaxel/MAD
- Merge pull request [#1](https://github.com/montanaflynn/stats/issues/1) from montanaflynn/master
- Merge pull request [#11](https://github.com/montanaflynn/stats/issues/11) from Kunde21/modeMemReduce
- Merge pull request [#10](https://github.com/montanaflynn/stats/issues/10) from Kunde21/ModeRewrite


<a name="0.2.0"></a>
## [0.2.0] - 2015-10-14
### Add
- Add Makefile with gometalinter, testing, benchmarking and coverage report targets
- Add comments describing functions and structs
- Add Correlation func
- Add Covariance func
- Add tests for new function shortcuts
- Add StandardDeviation function as a shortcut to StandardDeviationPopulation
- Add Float64Data and Series types

### Change
- Change Sample to return a standard []float64 type

### Fix
- Fix broken link to Makefile
- Fix broken link and simplify code coverage reporting command
- Fix go vet warning about printf type placeholder
- Fix failing codecov test coverage reporting
- Fix link to CHANGELOG.md

### Fixed
- Fixed typographical error, changed accomdate to accommodate in README.

### Include
- Include Variance and StandardDeviation shortcuts

### Pass
- Pass gometalinter

### Refactor
- Refactor Variance function to be the same as population variance

### Release
- Release version 0.2.0

### Remove
- Remove unneeded do packages and update cover URL
- Remove sudo from pip install

### Reorder
- Reorder functions and sections

### Revert
- Revert to legacy containers to preserve go1.1 testing

### Switch
- Switch from legacy to container-based CI infrastructure

### Update
- Update contributing instructions and mention Makefile

### Pull Requests
- Merge pull request [#5](https://github.com/montanaflynn/stats/issues/5) from orthographic-pedant/spell_check/accommodate


<a name="0.1.0"></a>
## [0.1.0] - 2015-08-19
### Add
- Add CONTRIBUTING.md

### Rename
- Rename functions while preserving backwards compatibility


<a name="0.0.9"></a>
## 0.0.9 - 2015-08-18
### Add
- Add HarmonicMean func
- Add GeometricMean func
- Add .gitignore to avoid commiting test coverage report
- Add Outliers stuct and QuantileOutliers func
- Add Interquartile Range, Midhinge and Trimean examples
- Add Trimean
- Add Midhinge
- Add Inter Quartile Range
- Add a unit test to check for an empty slice error
- Add Quantiles struct and Quantile func
- Add more tests and fix a typo
- Add Golang 1.5 to build tests
- Add a standard MIT license file
- Add basic benchmarking
- Add regression models
- Add codecov token
- Add codecov
- Add check for slices with a single item
- Add coverage tests
- Add back previous Go versions to Travis CI
- Add Travis CI
- Add GoDoc badge
- Add Percentile and Float64ToInt functions
- Add another rounding test for whole numbers
- Add build status badge
- Add code coverage badge
- Add test for NaN, achieving 100% code coverage
- Add round function
- Add standard deviation function
- Add sum function

### Add
- add tests for sample
- add sample

### Added
- Added sample and population variance and deviation functions
- Added README

### Adjust
- Adjust API ordering

### Avoid
- Avoid unintended consequence of using sort

### Better
- Better performing min/max
- Better description

### Change
- Change package path to potentially fix a bug in earlier versions of Go

### Clean
- Clean up README and add some more information
- Clean up test error

### Consistent
- Consistent empty slice error messages
- Consistent var naming
- Consistent func declaration

### Convert
- Convert ints to floats

### Duplicate
- Duplicate packages for all versions

### Export
- Export Coordinate struct fields

### First
- First commit

### Fix
- Fix copy pasta mistake testing the wrong function
- Fix error message
- Fix usage output and edit API doc section
- Fix testing edgecase where map was in wrong order
- Fix usage example
- Fix usage examples

### Include
- Include the Nearest Rank method of calculating percentiles

### More
- More commenting

### Move
- Move GoDoc link to top

### Redirect
- Redirect kills newer versions of Go

### Refactor
- Refactor code and error checking

### Remove
- Remove unnecassary typecasting in sum func
- Remove cover since it doesn't work for later versions of go
- Remove golint and gocoveralls

### Rename
- Rename StandardDev to StdDev
- Rename StandardDev to StdDev

### Return
- Return errors for all functions

### Run
- Run go fmt to clean up formatting

### Simplify
- Simplify min/max function

### Start
- Start with minimal tests

### Switch
- Switch wercker to travis and update todos

### Table
- table testing style

### Update
- Update README and move the example main.go into it's own file
- Update TODO list
- Update README
- Update usage examples and todos

### Use
- Use codecov the recommended way
- Use correct string formatting types

### Pull Requests
- Merge pull request [#4](https://github.com/montanaflynn/stats/issues/4) from saromanov/sample


[Unreleased]: https://github.com/montanaflynn/stats/compare/v0.7.1...HEAD
[v0.7.1]: https://github.com/montanaflynn/stats/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/montanaflynn/stats/compare/v0.6.6...v0.7.0
[v0.6.6]: https://github.com/montanaflynn/stats/compare/v0.6.5...v0.6.6
[v0.6.5]: https://github.com/montanaflynn/stats/compare/v0.6.4...v0.6.5
[v0.6.4]: https://github.com/montanaflynn/stats/compare/v0.6.3...v0.6.4
[v0.6.3]: https://github.com/montanaflynn/stats/compare/v0.6.2...v0.6.3
[v0.6.2]: https://github.com/montanaflynn/stats/compare/v0.6.1...v0.6.2
[v0.6.1]: https://github.com/montanaflynn/stats/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/montanaflynn/stats/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/montanaflynn/stats/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/montanaflynn/stats/compare/0.3.0...v0.4.0
[0.3.0]: https://github.com/montanaflynn/stats/compare/0.2.0...0.3.0
[0.2.0]: https://github.com/montanaflynn/stats/compare/0.1.0...0.2.0
[0.1.0]: https://github.com/montanaflynn/stats/compare/0.0.9...0.1.0
