## [HyperKit](http://github.com/docker/hyperkit)

![Build Status OSX](https://circleci.com/gh/docker/hyperkit.svg?style=shield&circle-token=cf8379b302eab2bbf33821cafe164dbefb71982d)

*HyperKit* is a toolkit for embedding hypervisor capabilities in your application. It includes a complete hypervisor, based on [xhyve](https://github.com/mist64/xhyve)/[bhyve](http://bhyve.org), which is optimized for lightweight virtual machines and container deployment.  It is designed to be interfaced with higher-level components such as the [VPNKit](https://github.com/docker/vpnkit) and [DataKit](https://github.com/docker/datakit).

HyperKit currently only supports Mac OS X using the [Hypervisor.framework](https://developer.apple.com/library/mac/documentation/DriversKernelHardware/Reference/Hypervisor/index.html). It is a core component of Docker For Mac.


## Requirements

* OS X 10.10.3 Yosemite or later
* a 2010 or later Mac (i.e. a CPU that supports EPT)

## Reporting Bugs

If you are using a version of Hyperkit which is embedded into a higher level application (e.g. [Docker for Mac](https://github.com/docker/for-mac)) then please report any issues against that higher level application in the first instance. That way the relevant team can triage and determine if the issue lies in Hyperkit and assign as necessary.

If you are using Hyperkit directly then please report issues against this repository.

## Usage

    $ hyperkit -h

## Building

    $ git clone https://github.com/docker/hyperkit
    $ cd hyperkit
    $ make

The resulting binary will be in `build/hyperkit`

To enable qcow support in the block backend an OCaml [OPAM](https://opam.ocaml.org) development
environment is required with the qcow module available. A
suitable environment can be setup by installing `opam` and `libev`
via `brew` and using `opam` to install the appropriate libraries:

    $ brew install opam libev
    $ opam init
    $ eval `opam config env`
    $ opam install uri qcow.0.8.1 mirage-block-unix.2.6.0 conf-libev logs fmt mirage-unix

Notes:

- `opam config env` must be evaluated each time prior to building
  hyperkit so the build will find the ocaml environment.
- Any previous pin of `mirage-block-unix` or `qcow`
  should be removed with the commands:
  
  ```sh
  $ opam update
  $ opam pin remove mirage-block-unix
  $ opam pin remove qcow
  ```

## Tracing

HyperKit defines a number of static DTrace probes to simplify investigation of
performance problems. To list the probes supported by your version of HyperKit,
type the following command while HyperKit VM is running:

     $ sudo dtrace -l -P 'hyperkit$target' -p $(pgrep hyperkit)

Refer to scripts in dtrace/ directory for examples of possible usage and
available probes.

### Relationship to xhyve and bhyve

HyperKit includes a hypervisor derived from [xhyve](http://www.xhyve.org), which in turn
was derived from [bhyve](http://www.bhyve.org). See the [original xhyve
README](README.xhyve.md) which incorporates the bhyve README.

We try to avoid deviating from these upstreams unnecessarily in order
to more easily share code, for example the various device
models/emulations should be easily shareable.

### Reporting security issues

The maintainers take security seriously. If you discover a security issue,
please bring it to their attention right away!

Please **DO NOT** file a public issue, instead send your report privately to
[security@docker.com](mailto:security@docker.com).

Security reports are greatly appreciated and we will publicly thank you for it.
We also like to send gifts&mdash;if you're into Docker schwag, make sure to let
us know. We currently do not offer a paid security bounty program, but are not
ruling it out in the future.


## Copyright and license

Copyright the authors and contributors. See individual source files
for details.

 Redistribution and use in source and binary forms, with or without
 modification, are permitted provided that the following conditions
 are met:
 1. Redistributions of source code must retain the above copyright
    notice, this list of conditions and the following disclaimer.
 2. Redistributions in binary form must reproduce the above copyright
    notice, this list of conditions and the following disclaimer in the
    documentation and/or other materials provided with the distribution.

 THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
 FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 SUCH DAMAGE.
