This directory holds integration tests written with the 
[bats](https://github.com/sstephenson/bats) bash testing framework.

These test upgrades from old docker versions to new versions via installing
docker from our apt repo. This way we can spot any problems people may have
when doing an `apt-get upgrade`.

Once our yum repo is live we will also accept tests for `yum upgrade`.

Installing bats is as simple as:

```console
$ git clone https://github.com/sstephenson/bats.git
$ cd bats
$ ./install.sh /usr/local
```

To run the tests:

```console
# enter the directory where docker is cloned
$ cd docker

$ make test-upgrade
# OR
$ ./hack/tests/run.sh
```

**WARNING:** running these tests on your host WILL uninstall and reinstall
various docker versions _ON YOUR HOST_.
