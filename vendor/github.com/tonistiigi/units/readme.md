#### Simple byte size formatting.

This package implements types that can be used in stdlib formatting functions
like `fmt.Printf` to control the output of the expected printed string.

Floating point flags `%f` and %g print the value in using the correct unit
suffix. Decimal units are default, `#` switches to binary units. If a value is
best represented as full bytes, integer bytes are printed instead.

##### Examples:

```
fmt.Printf("%.2f", 123 * B)   => "123B"
fmt.Printf("%.2f", 1234 * B)  => "1.23kB"
fmt.Printf("%g", 1200 * B)    => "1.2kB"
fmt.Printf("%#g", 1024 * B)   => "1KiB"
```


Integer flag `%d` always prints the value in bytes. `#` flag adds an unit prefix.

##### Examples:

```
fmt.Printf("%d", 1234 * B)    => "1234"
fmt.Printf("%#d", 1234 * B)   => "1234B"
```

`%v` is equal to `%g`