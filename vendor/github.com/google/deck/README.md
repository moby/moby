# Deck - Flexible Logging Framework for Go

The Deck package provides a flexible logging framework for Go apps. Deck
supports Windows Event Log, syslog, Go's standard logging, logging to file, and
log replays for testing. Deck is extensible, so new log backends can integrate
seamlessly into existing application code.

## Standard Logging

Deck can be a drop in replacement for some other logging packages, with support
for common functions like Info, Error, & Warning. The standard logging functions
write their outputs immediately and don't support additional attributes.

```
deck.Info("an info message")
```

```
deck.Errorf("an %v message", errors.New("error!"))
```

## Extensible Logging with Attributes

Sometimes we want to be able to pass more complex inputs to our log messages.
Deck facilitates this by allowing custom attributes to be attached to log
messages.

When using the `A`ttribute-supporting log functions, the message isn't output
immediately. We use the `With()` function to attach additional metadata to the
message first. The metadata can be anything supported by an attached
[backend](#backends). Once fully marked up with attributes, the `Go()` function
then performs the final write of the message.

In this example, a log message is marked with *Verbosity* level 2.
[Verbosity](#message-verbosity) can be used to dynamically show or hide log events at
runtime.

```
deck.InfoA("a verbose message").With(deck.V(2)).Go()
```

The EventLog backend for Windows supports Event IDs:

```
deck.InfoA("a windows event").With(eventlog.EventID(123)).Go()
```

Multiple attributes can be attributed to the same message:

```
deck.InfoA("a verbose windows event").With(eventlog.EventID(123), deck.V(3)).Go()
```

## Backends

Deck's logging functionality revolves around **backends**. A backend is any
logging destination that deck should write messages to. Backends are
plug-and-play, so you can reconfigure your application's logging behavior simply
by adding and removing different backends.

```
import (
  "github.com/google/deck"
  "github.com/google/deck/backends/logger"
)

deck.Add(logger.Init(os.Stdout, 0))
```

Cross-platform builds can support platform-specific log outputs by calling `Add`
from platform-specific source files.

```
// my_app_windows.go

func init() {
  evt, err := eventlog.Init("My App")
  if err != nil {
    panic(err)
  }
  deck.Add(evt)
}
```

```
// my_app_linux.go

func init() {
  sl, err := syslog.Init("my-app", syslog.LOG_USER)
  if err != nil {
    panic(err)
  }
  deck.Add(sl)
}
```

### eventlog Backend

The eventlog backend is for Windows only. This backend supports logging to the
Windows Event Log. It exports the `EventID` attribute that allows logging
messages with custom Event IDs.

### logger Backend

The logger backend is based on Go's core `log` package. It can take any
io.Writer, including os.Stdout, os.Stderr, io.Multiwriter, and any open file
handles.

### syslog Backend

The syslog backend is based on Go's core `syslog` package for Linux/Unix.

### discard Backend

The discard backend discards all log events. Deck requires at least one backend to be registered to handle logs. To suppress all output, add the discard backend.

## Message Verbosity

Verbosity is a special attribute implemented by the deck core package. The `V()`
function decorates logs with a custom verbosity, and the `SetVerbosity()`
function determines which verbosity levels get output. This allows the verbosity
level to be changed at runtime, such as via a flag or setting.

```
deck.SetVerbosity(*verbosityFlag)
...
log.InfoA("a level one message").With(deck.V(1)).Go()
log.InfoA("a level three message").With(deck.V(3)).Go()
```

In this example, if verbosityFlag is 2 or lower, only *"a level one message"*
will print. If it's 3 or higher, both messages will print. Verbosity defaults to
0, and all non-`A`ttribute functions will be at verbosity 0.

## Custom Decks

The `deck` package builds a global deck whenever it's imported, and most
implementations can just use this deck directly via the package-level logging
functions. For more advanced use cases, multiple decks can be constructed using
`deck.New()`. Each deck can have its own set of attached backends, and supports
the same functionality as the global deck.
