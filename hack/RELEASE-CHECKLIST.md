# Release Checklist
## A maintainer's guide to releasing Docker

So you're in charge of a Docker release? Cool. Here's what to do.

If your experience deviates from this document, please document the changes
to keep it up-to-date.

It is important to note that this document assumes that the git remote in your
repository that corresponds to "https://github.com/docker/docker" is named
"origin".  If yours is not (for example, if you've chosen to name it "upstream"
or something similar instead), be sure to adjust the listed snippets for your
local environment accordingly.  If you are not sure what your upstream remote is
named, use a command like `git remote -v` to find out.

If you don't have an upstream remote, you can add one easily using something
like:

```bash
export GITHUBUSER="YOUR_GITHUB_USER"
git remote add origin https://github.com/docker/docker.git
git remote add $GITHUBUSER git@github.com:$GITHUBUSER/docker.git
```

### 1. Pull from master and create a release branch

Note: Even for major releases, all of X, Y and Z in vX.Y.Z must be specified (e.g. v1.0.0).

```bash
export VERSION=vX.Y.Z
git fetch origin
git branch -D release || true
git checkout --track origin/release
git checkout -b bump_$VERSION
```

If it's a regular release, we usually merge master.
```bash
git merge origin/master
```

Otherwise, if it is a hotfix release, we cherry-pick only the commits we want.
```bash
# get the commits ids we want to cherry-pick
git log
# cherry-pick the commits starting from the oldest one, without including merge commits
git cherry-pick <commit-id>
git cherry-pick <commit-id>
...
```

### 2. Update CHANGELOG.md

You can run this command for reference with git 2.0:

```bash
git fetch --tags
LAST_VERSION=$(git tag -l --sort=-version:refname "v*" | grep -E 'v[0-9\.]+$' | head -1)
git log --stat $LAST_VERSION..bump_$VERSION
```

If you don't have git 2.0 but have a sort command that supports `-V`:
```bash
git fetch --tags
LAST_VERSION=$(git tag -l | grep -E 'v[0-9\.]+$' | sort -rV | head -1)
git log --stat $LAST_VERSION..bump_$VERSION
```

If releasing a major version (X or Y increased in vX.Y.Z), simply listing notable user-facing features is sufficient.
```markdown
#### Notable features since <last major version>
* New docker command to do something useful
* Remote API change (deprecating old version)
* Performance improvements in some usecases
* ...
```

For minor releases (only Z increases in vX.Y.Z), provide a list of user-facing changes.
Each change should be listed under a category heading formatted as `#### CATEGORY`.

`CATEGORY` should describe which part of the project is affected.
  Valid categories are:
  * Builder
  * Documentation
  * Hack
  * Packaging
  * Remote API
  * Runtime
  * Other (please use this category sparingly)

Each change should be formatted as `BULLET DESCRIPTION`, given:

* BULLET: either `-`, `+` or `*`, to indicate a bugfix, new feature or
  upgrade, respectively.

* DESCRIPTION: a concise description of the change that is relevant to the
  end-user, using the present tense. Changes should be described in terms
  of how they affect the user, for example "Add new feature X which allows Y",
  "Fix bug which caused X", "Increase performance of Y".

EXAMPLES:

```markdown
## 0.3.6 (1995-12-25)

#### Builder

+ 'docker build -t FOO .' applies the tag FOO to the newly built image

#### Remote API

- Fix a bug in the optional unix socket transport

#### Runtime

* Improve detection of kernel version
```

If you need a list of contributors between the last major release and the
current bump branch, use something like:
```bash
git log --format='%aN <%aE>' v0.7.0...bump_v0.8.0 | sort -uf
```
Obviously, you'll need to adjust version numbers as necessary.  If you just need
a count, add a simple `| wc -l`.

### 3. Change the contents of the VERSION file

```bash
echo ${VERSION#v} > VERSION
```

### 4. Test the docs

Make sure that your tree includes documentation for any modified or
new features, syntax or semantic changes.

To test locally:

```bash
make docs
```

To make a shared test at http://beta-docs.docker.io:

(You will need the `awsconfig` file added to the `docs/` dir)

```bash
make AWS_S3_BUCKET=beta-docs.docker.io BUILD_ROOT=yes docs-release
```

### 5. Commit and create a pull request to the "release" branch

```bash
git add VERSION CHANGELOG.md
git commit -m "Bump version to $VERSION"
git push $GITHUBUSER bump_$VERSION
echo "https://github.com/$GITHUBUSER/docker/compare/docker:release...$GITHUBUSER:bump_$VERSION?expand=1"
```

That last command will give you the proper link to visit to ensure that you
open the PR against the "release" branch instead of accidentally against
"master" (like so many brave souls before you already have).

### 6. Get 2 other maintainers to validate the pull request

### 7. Publish binaries

To run this you will need access to the release credentials. Get them from the Core maintainers.

Replace "..." with the respective credentials:

```bash
docker build -t docker .
docker run \
       -e AWS_S3_BUCKET=test.docker.com \
       -e AWS_ACCESS_KEY="..." \
       -e AWS_SECRET_KEY="..." \
       -e GPG_PASSPHRASE="..." \
       -i -t --privileged \
       docker \
       hack/release.sh
```

It will run the test suite, build the binaries and packages,
and upload to the specified bucket (you should use test.docker.com for
general testing, and once everything is fine, switch to get.docker.com as
noted below).

After the binaries and packages are uploaded to test.docker.com, make sure
they get tested in both Ubuntu and Debian for any obvious installation
issues or runtime issues.

Announcing on IRC in both `#docker` and `#docker-dev` is a great way to get
help testing!  An easy way to get some useful links for sharing:

```bash
echo "Ubuntu/Debian: https://test.docker.com/ubuntu or curl -sSL https://test.docker.com/ | sh"
echo "Linux 64bit binary: https://test.docker.com/builds/Linux/x86_64/docker-${VERSION#v}"
echo "Darwin/OSX 64bit client binary: https://test.docker.com/builds/Darwin/x86_64/docker-${VERSION#v}"
echo "Darwin/OSX 32bit client binary: https://test.docker.com/builds/Darwin/i386/docker-${VERSION#v}"
echo "Linux 64bit tgz: https://test.docker.com/builds/Linux/x86_64/docker-${VERSION#v}.tgz"
```

Once they're tested and reasonably believed to be working, run against
get.docker.com:

```bash
docker run \
       -e AWS_S3_BUCKET=get.docker.com \
       -e AWS_ACCESS_KEY="..." \
       -e AWS_SECRET_KEY="..." \
       -e GPG_PASSPHRASE="..." \
       -i -t --privileged \
       docker \
       hack/release.sh
```

### 8. Breakathon

Spend several days along with the community explicitly investing time and
resources to try and break Docker in every possible way, documenting any
findings pertinent to the release.  This time should be spent testing and
finding ways in which the release might have caused various features or upgrade
environments to have issues, not coding.  During this time, the release is in
code freeze, and any additional code changes will be pushed out to the next
release.

It should include various levels of breaking Docker, beyond just using Docker
by the book.

Any issues found may still remain issues for this release, but they should be
documented and give appropriate warnings.

### 9. Apply tag

It's very important that we don't make the tag until after the official
release is uploaded to get.docker.com!

```bash
git tag -a $VERSION -m $VERSION bump_$VERSION
git push origin $VERSION
```

### 10. Go to github to merge the `bump_$VERSION` branch into release

Don't forget to push that pretty blue button to delete the leftover
branch afterwards!

### 11. Update the docs branch

If this is a MAJOR.MINOR.0 release, you need to make an branch for the previous release's
documentation:

```bash
git checkout -b docs-$PREVIOUS_MAJOR_MINOR docs
git fetch
git reset --hard origin/docs
git push -f origin docs-$PREVIOUS_MAJOR_MINOR
```

You will need the `awsconfig` file added to the `docs/` directory to contain the
s3 credentials for the bucket you are deploying to.

```bash
git checkout -b docs release || git checkout docs
git fetch
git reset --hard origin/release
git push -f origin docs
make AWS_S3_BUCKET=docs.docker.com BUILD_ROOT=yes docs-release
```

The docs will appear on http://docs.docker.com/ (though there may be cached
versions, so its worth checking http://docs.docker.com.s3-website-us-east-1.amazonaws.com/).
For more information about documentation releases, see `docs/README.md`.

Ask Sven, or JohnC to invalidate the cloudfront cache using the CND Planet chrome applet.

### 12. Create a new pull request to merge release back into master

```bash
git checkout master
git fetch
git reset --hard origin/master
git merge origin/release
git checkout -b merge_release_$VERSION
echo ${VERSION#v}-dev > VERSION
git add VERSION
git commit -m "Change version to $(cat VERSION)"
git push $GITHUBUSER merge_release_$VERSION
echo "https://github.com/$GITHUBUSER/docker/compare/docker:master...$GITHUBUSER:merge_release_$VERSION?expand=1"
```

Again, get two maintainers to validate, then merge, then push that pretty
blue button to delete your branch.

### 13. Rejoice and Evangelize!

Congratulations! You're done.

Go forth and announce the glad tidings of the new release in `#docker`,
`#docker-dev`, on the [mailing list](https://groups.google.com/forum/#!forum/docker-dev),
and on Twitter!
