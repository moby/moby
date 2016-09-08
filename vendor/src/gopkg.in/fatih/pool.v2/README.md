# Pool [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/gopkg.in/fatih/pool.v2) [![Build Status](http://img.shields.io/travis/fatih/pool.svg?style=flat-square)](https://travis-ci.org/fatih/pool)


Pool is a thread safe connection pool for net.Conn interface. It can be used to
manage and reuse connections.


## Install and Usage

Install the package with:

```bash
go get gopkg.in/fatih/pool.v2
```

Import it with:

```go
import "gopkg.in/fatih/pool.v2"
```

and use `pool` as the package name inside the code.

## Example

```go
// create a factory() to be used with channel based pool
factory    := func() (net.Conn, error) { return net.Dial("tcp", "127.0.0.1:4000") }

// create a new channel based pool with an initial capacity of 5 and maximum
// capacity of 30. The factory will create 5 initial connections and put it
// into the pool.
p, err := pool.NewChannelPool(5, 30, factory)

// now you can get a connection from the pool, if there is no connection
// available it will create a new one via the factory function.
conn, err := p.Get()

// do something with conn and put it back to the pool by closing the connection
// (this doesn't close the underlying connection instead it's putting it back
// to the pool).
conn.Close()

// close pool any time you want, this closes all the connections inside a pool
p.Close()

// currently available connections in the pool
current := p.Len()
```


## Credits

 * [Fatih Arslan](https://github.com/fatih)
 * [sougou](https://github.com/sougou)

## License

The MIT License (MIT) - see LICENSE for more details
