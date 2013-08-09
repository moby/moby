## A maintainer's guide to releasing Docker

So you're in charge of a docker release? Cool. Here's what to do.

If your experience deviates from this document, please document the changes to keep it
up-to-date.


### 1. Pull from master and create a release branch

	```bash
	$ git checkout master
	$ git pull
	$ git checkout -b bump_$VERSION
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

	* DESCRIPTION: a concise description of the change that is relevant to the end-user,
	using the present tense.
	Changes should be described in terms of how they affect the user, for example "new feature
	X which allows Y", "fixed bug which caused X", "increased performance of Y".

	EXAMPLES:

		```
		 + Builder: 'docker build -t FOO' applies the tag FOO to the newly built container.
		 * Runtime: improve detection of kernel version
		 - Remote API: fix a bug in the optional unix socket transport
		 ```

### 3. Change the contents of the VERSION file

### 4. Run all tests

	```bash
	$ make test
	```

### 5. Commit and create a pull request

	```bash
	$ git add commands.go CHANGELOG.md
	$ git commit -m "Bump version to $VERSION"
	$ git push origin bump_$VERSION
	```

### 6. Get 2 other maintainers to validate the pull request

### 7. Merge the pull request and apply tags

	```bash
	$ git checkout master
	$ git merge bump_$VERSION
	$ git tag -a v$VERSION # Don't forget the v!
	$ git tag -f -a latest
	$ git push
	$ git push --tags
	```

### 8. Publish binaries

	To run this you will need access to the release credentials.
	Get them from [the infrastructure maintainers](https://github.com/dotcloud/docker/blob/master/hack/infrastructure/MAINTAINERS).

	```bash
	$ RELEASE_IMAGE=image_provided_by_infrastructure_maintainers
	$ BUILD=$(docker run -d -e RELEASE_PPA=0 $RELEASE_IMAGE)
	```

	This will do 2 things:
	
	* It will build and upload the binaries on http://get.docker.io
	* It will *test* the release on our Ubuntu PPA (a PPA is a community repository for ubuntu packages)

	Wait for the build to complete.

	```bash
	$ docker wait $BUILD # This should print 0. If it doesn't, your build failed.
	```

	Check that the output looks OK. Here's an example of a correct output:

	```bash
	$ docker logs 2>&1 b4e7c8299d73 | grep -e 'Public URL' -e 'Successfully uploaded'
	Public URL of the object is: http://get.docker.io.s3.amazonaws.com/builds/Linux/x86_64/docker-v0.4.7.tgz
	Public URL of the object is: http://get.docker.io.s3.amazonaws.com/builds/Linux/x86_64/docker-latest.tgz
	Successfully uploaded packages.
	```

	If you don't see 3 lines similar to this, something might be wrong. Check the full logs and try again.
	

### 9. Publish Ubuntu packages

	If everything went well in the previous step, you can finalize the release by submitting the Ubuntu
	packages.

	```bash
	$ RELEASE_IMAGE=image_provided_by_infrastructure_maintainers
	$ docker run -e RELEASE_PPA=1 $RELEASE_IMAGE
	```

	If that goes well, Ubuntu Precise package is in its way. It will take anywhere from 0.5 to 30 hours
	for the builders to complete their job depending on builder demand at this time. At this point, Quantal
	and Raring packages need to be created using the Launchpad interface:
	  https://launchpad.net/~dotcloud/+archive/lxc-docker/+packages

	Notify [the packager maintainers](https://github.com/dotcloud/docker/blob/master/packaging/MAINTAINERS)
	who will ensure PPA is ready.

	Congratulations! You're done
