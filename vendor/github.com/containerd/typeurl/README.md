# typeurl

[![Build Status](https://travis-ci.org/containerd/typeurl.svg?branch=master)](https://travis-ci.org/containerd/typeurl)

[![codecov](https://codecov.io/gh/containerd/typeurl/branch/master/graph/badge.svg)](https://codecov.io/gh/containerd/typeurl)

A Go package for managing the registration, marshaling, and unmarshaling of encoded types.

This package helps when types are sent over a GRPC API and marshaled as a [protobuf.Any]().
