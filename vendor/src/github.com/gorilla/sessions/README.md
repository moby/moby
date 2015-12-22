sessions
========
[![GoDoc](https://godoc.org/github.com/gorilla/sessions?status.svg)](https://godoc.org/github.com/gorilla/sessions) [![Build Status](https://travis-ci.org/gorilla/sessions.png?branch=master)](https://travis-ci.org/gorilla/sessions)

gorilla/sessions provides cookie and filesystem sessions and infrastructure for
custom session backends.

The key features are:

* Simple API: use it as an easy way to set signed (and optionally
  encrypted) cookies.
* Built-in backends to store sessions in cookies or the filesystem.
* Flash messages: session values that last until read.
* Convenient way to switch session persistency (aka "remember me") and set
  other attributes.
* Mechanism to rotate authentication and encryption keys.
* Multiple sessions per request, even using different backends.
* Interfaces and infrastructure for custom session backends: sessions from
  different stores can be retrieved and batch-saved using a common API.

Let's start with an example that shows the sessions API in a nutshell:

```go
	import (
		"net/http"
		"github.com/gorilla/sessions"
	)

	var store = sessions.NewCookieStore([]byte("something-very-secret"))

	func MyHandler(w http.ResponseWriter, r *http.Request) {
		// Get a session. We're ignoring the error resulted from decoding an
		// existing session: Get() always returns a session, even if empty.
		session, _ := store.Get(r, "session-name")
		// Set some session values.
		session.Values["foo"] = "bar"
		session.Values[42] = 43
		// Save it before we write to the response/return from the handler.
		session.Save(r, w)
	}
```

First we initialize a session store calling NewCookieStore() and passing a
secret key used to authenticate the session. Inside the handler, we call
store.Get() to retrieve an existing session or a new one. Then we set some
session values in session.Values, which is a map[interface{}]interface{}.
And finally we call session.Save() to save the session in the response.

More examples are available [on the Gorilla
website](http://www.gorillatoolkit.org/pkg/sessions).

## Store Implementations

Other implementations of the sessions.Store interface:

 * [github.com/starJammer/gorilla-sessions-arangodb](https://github.com/starJammer/gorilla-sessions-arangodb) - ArangoDB
 * [github.com/yosssi/boltstore](https://github.com/yosssi/boltstore) - Bolt
 * [github.com/srinathgs/couchbasestore](https://github.com/srinathgs/couchbasestore) - Couchbase
 * [github.com/denizeren/dynamostore](https://github.com/denizeren/dynamostore) - Dynamodb on AWS
 * [github.com/bradleypeabody/gorilla-sessions-memcache](https://github.com/bradleypeabody/gorilla-sessions-memcache) - Memcache
 * [github.com/hnakamur/gaesessions](https://github.com/hnakamur/gaesessions) - Memcache on GAE
 * [github.com/kidstuff/mongostore](https://github.com/kidstuff/mongostore) - MongoDB
 * [github.com/srinathgs/mysqlstore](https://github.com/srinathgs/mysqlstore) - MySQL
 * [github.com/antonlindstrom/pgstore](https://github.com/antonlindstrom/pgstore) - PostgreSQL
 * [github.com/boj/redistore](https://github.com/boj/redistore) - Redis
 * [github.com/boj/rethinkstore](https://github.com/boj/rethinkstore) - RethinkDB
 * [github.com/boj/riakstore](https://github.com/boj/riakstore) - Riak
 * [github.com/michaeljs1990/sqlitestore](https://github.com/michaeljs1990/sqlitestore) - SQLite

 ## License

 BSD licensed. See the LICENSE file for details.
