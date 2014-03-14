# Contributing

### Moving Notice

We plan to include os/fsnotify in the Go standard library with a new [API](http://goo.gl/MrYxyA). 

* Import `code.google.com/p/go.exp/fsnotify` ([GoDoc](http://godoc.org/code.google.com/p/go.exp/fsnotify)) for the latest API under development.
* Continue importing `github.com/howeyc/fsnotify` ([GoDoc](http://godoc.org/github.com/howeyc/fsnotify)) for the stable API.
* [Report Issues](https://code.google.com/p/go/issues/list?q=fsnotify) to go.exp/fsnotify after testing against `code.google.com/p/go.exp/fsnotify`
* Join [golang-dev](https://groups.google.com/forum/#!forum/golang-dev) to discuss fsnotify.
* See the [Contribution Guidelines](http://golang.org/doc/contribute.html) for Go and sign the CLA.

### Pull Requests

To hack on fsnotify:

1. Install as usual (`go get -u github.com/howeyc/fsnotify`)
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Ensure everything works and the tests pass (see below)
4. Commit your changes (`git commit -am 'Add some feature'`)

Contribute upstream:

1. Fork fsnotify on GitHub
2. Add your remote (`git remote add fork git@github.com:mycompany/repo.git`)
3. Push to the branch (`git push fork my-new-feature`)
4. Create a new Pull Request on GitHub

For other team members:

1. Install as usual (`go get -u github.com/howeyc/fsnotify`)
2. Add your remote (`git remote add fork git@github.com:mycompany/repo.git`)
3. Pull your revisions (`git fetch fork; git checkout -b my-new-feature fork/my-new-feature`)

Notice: Always use the original import path by installing with `go get`.

### Testing

fsnotify uses build tags to compile different code on Linux, BSD, OS X, and Windows. Our continuous integration server is only able to test on Linux at this time.

Before doing a pull request, please do your best to test your changes on multiple platforms, and list which platforms you were able/unable to test on.

To make cross-platform testing easier, we've created a Vagrantfile for Linux and BSD.

* Install [Vagrant](http://www.vagrantup.com/) and [VirtualBox](https://www.virtualbox.org/)
* Setup [Vagrant Gopher](https://github.com/gophertown/vagrant-gopher) in your `src` folder.
* Run `vagrant up` from the project folder. You can also setup just one box with `vagrant up linux` or `vagrant up bsd` (note: the BSD box doesn't support Windows hosts at this time, and NFS may prompt for your host OS password)
* Once setup, you can run the test suite on a given OS with a single command `vagrant ssh linux -c 'cd howeyc/fsnotify; go test ./...'`.
* When you're done, you will want to halt or destroy the vagrant boxes.

Notice: fsnotify file system events won't work on shared folders. The tests get around this limitation by using a tmp directory, but it is something to be aware of when logging in with `vagrant ssh linux` to do some manual testing.

Right now we don't have an equivalent solution for Windows and OS X, but there are Windows VMs [freely available from Microsoft](http://www.modern.ie/en-us/virtualization-tools#downloads).
