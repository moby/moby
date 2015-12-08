# srslog

Go has a `syslog` package in the standard library, but it has the following
shortcomings:

1. It doesn't have TLS support
2. [According to bradfitz on the Go team, it is no longer being maintained.](https://github.com/golang/go/issues/13449#issuecomment-161204716)

I agree that it doesn't need to be in the standard library. So, I've
followed Brad's suggestion and have made a separate project to handle syslog.

This code was taken directly from the Go project as a base to start from.

However, this _does_ have TLS support.

# Usage

You can still use it for local syslog:

```
w, err := srslog.Dial("", "", srslog.LOG_ERR, "testtag")
```

Or to unencrypted UDP:

```
w, err := srslog.Dial("udp", "192.168.0.50:514", srslog.LOG_ERR, "testtag")
```

Or to unencrypted TCP:

```
w, err := srslog.Dial("tcp", "192.168.0.51:514", srslog.LOG_ERR, "testtag")
```

But now you can also send messages via TLS-encrypted TCP:

```
w, err := srslog.DialWithTLSCertPath("tcp+tls", "192.168.0.52:514", srslog.LOG_ERR, "testtag", "/path/to/servercert.pem")
```

And if you need more control over your TLS configuration :

```
pool := x509.NewCertPool()
serverCert, err := ioutil.ReadFile("/path/to/servercert.pem")
if err != nil {
    return nil, err
}
pool.AppendCertsFromPEM(serverCert)
config := tls.Config{
    RootCAs: pool,
}

w, err := DialWithTLSConfig(network, raddr, priority, tag, &config)
```

(Note that in both TLS cases, this uses a self-signed certificate, where the
remote syslog server has the keypair and the client has only the public key.)

And then to write log messages, continue like so:

```
if err != nil {
    log.Fatal("failed to connect to syslog:", err)
}
defer w.Close()

w.Alert("this is an alert")
w.Crit("this is critical")
w.Err("this is an error")
w.Warning("this is a warning")
w.Notice("this is a notice")
w.Info("this is info")
w.Debug("this is debug")
w.Write([]byte("these are some bytes"))
```

# Generating TLS Certificates

We've provided a script that you can use to generate a self-signed keypair:

```
pip install cryptography
python script/gen-certs.py
```

That outputs the public key and private key to standard out. Put those into
`.pem` files. (And don't put them into any source control.)

# Running Tests

Run the tests as usual:

```
go test
```

But we've also provided a test coverage script that will show you which
lines of code are not covered:

```
script/coverage --html
```

That will open a new browser tab showing coverage information.

# License

This project uses the New BSD License, the same as the Go project itself.
