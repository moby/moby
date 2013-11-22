## A maintainer's guide to releasing Docker

So you're in charge of a Docker release? Cool. Here's what to do.

If your experience deviates from this document, please document the changes
to keep it up-to-date.

### 1. Pull from master and create a release branch

```bash
export VERSION=vXXX
git checkout release
git pull
git checkout -b bump_$VERSION
git merge origin/master
```

### 2. Update CHANGELOG.md

You can run this command for reference:

```bash
LAST_VERSION=$(git tag | grep -E "v[0-9\.]+$" | sort -nr | head -n 1)
git log $LAST_VERSION..HEAD
```

Each change should be formatted as ```BULLET CATEGORY: DESCRIPTION```

* BULLET is either ```-```, ```+``` or ```*```, to indicate a bugfix,
  new feature or upgrade, respectively.

* CATEGORY should describe which part of the project is affected.
  Valid categories are:
  * Builder
  * Documentation
  * Hack
  * Packaging
  * Remote API
  * Runtime

* DESCRIPTION: a concise description of the change that is relevant to the 
  end-user, using the present tense. Changes should be described in terms 
  of how they affect the user, for example "new feature X which allows Y", 
  "fixed bug which caused X", "increased performance of Y".

EXAMPLES:

```
+ Builder: 'docker build -t FOO' applies the tag FOO to the newly built
  container.
* Runtime: improve detection of kernel version
- Remote API: fix a bug in the optional unix socket transport
```

### 3. Change the contents of the VERSION file

```bash
echo ${VERSION#v} > VERSION
```

### 4. Run all tests

```bash
docker run -privileged docker hack/make.sh test
```

### 5. Test the docs

Make sure that your tree includes documentation for any modified or
new features, syntax or semantic changes. Instructions for building
the docs are in ``docs/README.md``

### 6. Commit and create a pull request to the "release" branch

```bash
git add VERSION CHANGELOG.md
git commit -m "Bump version to $VERSION"
git push origin bump_$VERSION
```

### 7. Get 2 other maintainers to validate the pull request

### 8. Apply tag

```bash
git tag -a $VERSION -m $VERSION bump_$VERSION
git push origin $VERSION
```

Merging the pull request to the release branch will automatically
update the documentation on the "latest" revision of the docs. You
should see the updated docs 5-10 minutes after the merge. The docs
will appear on http://docs.docker.io/. For more information about
documentation releases, see ``docs/README.md``

### 9. Go to github to merge the bump_$VERSION into release

Don't forget to push that pretty blue button to delete the leftover
branch afterwards!

### 10. Publish binaries

To run this you will need access to the release credentials.
Get them from [the infrastructure maintainers](
https://github.com/dotcloud/docker/blob/master/hack/infrastructure/MAINTAINERS).

```bash
git checkout release
git fetch
git reset --hard origin/release
docker build -t docker .
docker run  \
       -e AWS_S3_BUCKET=test.docker.io \
       -e AWS_ACCESS_KEY=$(cat ~/.aws/access_key) \
       -e AWS_SECRET_KEY=$(cat ~/.aws/secret_key) \
       -e GPG_PASSPHRASE=supersecretsesame \
       -i -t -privileged \
       docker \
       hack/release.sh
```

It will run the test suite one more time, build the binaries and packages,
and upload to the specified bucket (you should use test.docker.io for
general testing, and once everything is fine, switch to get.docker.io).

### 11. Rejoice and Evangelize!

Congratulations! You're done.

Go forth and announce the glad tidings of the new release in `#docker`,
`#docker-dev`, on the [mailing list](https://groups.google.com/forum/#!forum/docker-dev),
and on Twitter!
