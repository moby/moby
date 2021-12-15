# how to contribute to the go zookeeper library

## **Did you find a bug?**

* **Ensure the bug was not already reported** by searching on GitHub under [Issues](https://github.com/go-zookeper/zk/issues).

* If you're unable to find an open issue addressing the problem, open a new one.
  * Be sure to include a title and clear description.
  * Be sure to include the actual behavior vs the expected.
  * As much relevant information as possible, a code sample or an executable test case demonstrating the expected vs actual behavior.

## Did you write a patch that fixes a bug

* Ensure that all bugs are first reported as an issue. This will help others in finding fixes through issues first.

* Open a PR referencing the issue for the bug.

## Pull Requests

We are open to all Pull Requests, its best to accompany the requests with an issue.

* The PR requires the github actions to pass.

* Requires at least one maintainer to approve the PR to merge to master.

While the above must be satisfied prior to having your pull request reviewed, the reviewer(s) may ask you to complete additional design work, tests, or other changes before your pull request can be ultimately accepted.

## Versioned Releases

Since this library is a core client for interacting with Zookeeper, we do [SemVer](https://semver.org/) releases to ensure predictable changes for users.

Zookeeper itself maintains a compatibility check on the main codebase as well as maintaining backwards compatibility through all Major releases, this core library will try to uphold similar standards of releases.

* Code that is merged into master should be ready for release at any given time.
  * This is to say, that code should not be merged into master if it is not complete and ready for production use.

* If a fix needs to be released ahead of normal operations, file an issue explaining the urgency and impact of the bug.

## Coding guidelines

Some good external resources for style:

1. [Effective Go](https://golang.org/doc/effective_go.html)
2. [The Go common mistakes guide](https://github.com/golang/go/wiki/CodeReviewComments)

All code should be error-free when run through `golint` and `go vet`. We
recommend setting up your editor to:

* Run `goimports` on save
* Run `golint` and `go vet` to check for errors

You can find information in editor support for Go tools here:
<https://github.com/golang/go/wiki/IDEsAndTextEditorPlugins>

## Addition information

* We have zero external dependencies, and would like to maintain this. Use of any external go library should be limited to tests.
