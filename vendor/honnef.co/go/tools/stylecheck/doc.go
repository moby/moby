package stylecheck

var docST1000 = `Incorrect or missing package comment

Packages must have a package comment that is formatted according to
the guidelines laid out in
https://github.com/golang/go/wiki/CodeReviewComments#package-comments.

Available since
    2019.1, non-default
`

var docST1001 = `Dot imports are discouraged

Dot imports that aren't in external test packages are discouraged.

The dot_import_whitelist option can be used to whitelist certain
imports.

Quoting Go Code Review Comments:

    The import . form can be useful in tests that, due to circular
    dependencies, cannot be made part of the package being tested:

        package foo_test

        import (
            "bar/testutil" // also imports "foo"
            . "foo"
        )

    In this case, the test file cannot be in package foo because it
    uses bar/testutil, which imports foo. So we use the 'import .'
    form to let the file pretend to be part of package foo even though
    it is not. Except for this one case, do not use import . in your
    programs. It makes the programs much harder to read because it is
    unclear whether a name like Quux is a top-level identifier in the
    current package or in an imported package.

Available since
    2019.1

Options
    dot_import_whitelist
`

var docST1003 = `Poorly chosen identifier

Identifiers, such as variable and package names, follow certain rules.

See the following links for details:

    http://golang.org/doc/effective_go.html#package-names
    http://golang.org/doc/effective_go.html#mixed-caps
    https://github.com/golang/go/wiki/CodeReviewComments#initialisms
    https://github.com/golang/go/wiki/CodeReviewComments#variable-names

Available since
    2019.1, non-default

Options
    initialisms
`

var docST1005 = `Incorrectly formatted error string

Error strings follow a set of guidelines to ensure uniformity and good
composability.

Quoting Go Code Review Comments:

    Error strings should not be capitalized (unless beginning with
    proper nouns or acronyms) or end with punctuation, since they are
    usually printed following other context. That is, use
    fmt.Errorf("something bad") not fmt.Errorf("Something bad"), so
    that log.Printf("Reading %s: %v", filename, err) formats without a
    spurious capital letter mid-message.

Available since
    2019.1
`

var docST1006 = `Poorly chosen receiver name

Quoting Go Code Review Comments:

    The name of a method's receiver should be a reflection of its
    identity; often a one or two letter abbreviation of its type
    suffices (such as "c" or "cl" for "Client"). Don't use generic
    names such as "me", "this" or "self", identifiers typical of
    object-oriented languages that place more emphasis on methods as
    opposed to functions. The name need not be as descriptive as that
    of a method argument, as its role is obvious and serves no
    documentary purpose. It can be very short as it will appear on
    almost every line of every method of the type; familiarity admits
    brevity. Be consistent, too: if you call the receiver "c" in one
    method, don't call it "cl" in another.

Available since
    2019.1
`

var docST1008 = `A function's error value should be its last return value

A function's error value should be its last return value.

Available since
    2019.1
`

var docST1011 = `Poorly chosen name for variable of type time.Duration

time.Duration values represent an amount of time, which is represented
as a count of nanoseconds. An expression like 5 * time.Microsecond
yields the value 5000. It is therefore not appropriate to suffix a
variable of type time.Duration with any time unit, such as Msec or
Milli.

Available since
    2019.1
`

var docST1012 = `Poorly chosen name for error variable

Error variables that are part of an API should be called errFoo or
ErrFoo.

Available since
    2019.1
`

var docST1013 = `Should use constants for HTTP error codes, not magic numbers

HTTP has a tremendous number of status codes. While some of those are
well known (200, 400, 404, 500), most of them are not. The net/http
package provides constants for all status codes that are part of the
various specifications. It is recommended to use these constants
instead of hard-coding magic numbers, to vastly improve the
readability of your code.

Available since
    2019.1

Options
    http_status_code_whitelist
`

var docST1015 = `A switch's default case should be the first or last case

Available since
    2019.1
`

var docST1016 = `Use consistent method receiver names

Available since
    2019.1, non-default
`

var docST1017 = `Don't use Yoda conditions

Available since
    Unreleased
`

var docST1018 = `Avoid zero-width and control characters in string literals

Available since
    Unreleased
`
