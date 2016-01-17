[![Build Status](https://travis-ci.org/moznion/go-unicode-east-asian-width.svg?branch=master)](https://travis-ci.org/moznion/go-unicode-east-asian-width)

# go-unicode-east-asian-width

Provides width properties of East Asian Characters.  
This package is port of [Unicode::EastAsianWidth](http://search.cpan.org/~audreyt/Unicode-EastAsianWidth/lib/Unicode/EastAsianWidth.pm) from Perl to Go.

[https://godoc.org/github.com/moznion/go-unicode-east-asian-width](https://godoc.org/github.com/moznion/go-unicode-east-asian-width)

## Getting Started

```
go get github.com/moznion/go-unicode-east-asian-width
```

## Synopsis

```go
import (
	"unicode"

	"github.com/moznion/go-unicode-east-asian-width"
)

func main() {
	eastasianwidth.IsFullwidth('世') // true
	eastasianwidth.IsHalfwidth('界') // false
	eastasianwidth.IsHalfwidth('w') // true

	unicode.Is(eastasianwidth.EastAsianWide, '禅') // true
}
```

## Provided Variables

- `EastAsian bool`

EastAsian is the flag for ambiguous character.  
If this flag is `true`, ambiguous characters are treated as full width.
Elsewise, ambiguous characters are treated as half width.  
Default value is `false`.

- `EastAsianAmbiguous unicode.RangeTable`
- `EastAsianFullwidth unicode.RangeTable`
- `EastAsianHalfwidth unicode.RangeTable`
- `EastAsianNarrow unicode.RangeTable`
- `EastAsianNeutral unicode.RangeTable`
- `EastAsianWide unicode.RangeTable`
- `Fullwidth unicode.RangeTable`
- `Halfwidth unicode.RangeTable`

[RangeTable](http://golang.org/pkg/unicode/#RangeTable) for East Asian characters.

## Functions

- `func Fullwidth() map[string]*unicode.RangeTable`

Returns the map of [RangeTable](http://golang.org/pkg/unicode/#RangeTable) of full width East Asian characters.

- `func Halfwidth() map[string]*unicode.RangeTable`

Returns the map of [RangeTable](http://golang.org/pkg/unicode/#RangeTable) of half width East Asian characters.

- `func IsFullwidth(r rune) bool`

Reports whether the rune is in range of full width character of East Asian.

- `func IsHalfwidth(r rune) bool`

Reports whether the rune is in range of half width character of East Asian.

## See Also

- [unicode](http://golang.org/pkg/unicode/)
- [Unicode::EastAsianWidth](http://search.cpan.org/~audreyt/Unicode-EastAsianWidth/lib/Unicode/EastAsianWidth.pm)

## LICENSE

MIT
