package simple

var docS1000 = `Use plain channel send or receive

Select statements with a single case can be replaced with a simple send or receive.

Before:

select {
case x := <-ch:
  fmt.Println(x)
}

After:

x := <-ch
fmt.Println(x)

Available since
    2017.1
`

var docS1001 = `Replace with copy()

Use copy() for copying elements from one slice to another.

Before:

for i, x := range src {
  dst[i] = x
}

After:

copy(dst, src)

Available since
    2017.1
`

var docS1002 = `Omit comparison with boolean constant

Before:

if x == true {}

After:

if x {}

Available since
    2017.1
`

var docS1003 = `Replace with strings.Contains

Before:

if strings.Index(x, y) != -1 {}

After:

if strings.Contains(x, y) {}

Available since
    2017.1
`

var docS1004 = `Replace with bytes.Equal

Before:

if bytes.Compare(x, y) == 0 {}

After:

if bytes.Equal(x, y) {}

Available since
    2017.1
`

var docS1005 = `Drop unnecessary use of the blank identifier

In many cases, assigning to the blank identifier is unnecessary.

Before:

for _ = range s {}
x, _ = someMap[key]
_ = <-ch

After:

for range s{}
x = someMap[key]
<-ch

Available since
    2017.1
`

var docS1006 = `Replace with for { ... }

For infinite loops, using for { ... } is the most idiomatic choice.

Available since
    2017.1
`

var docS1007 = `Simplify regular expression by using raw string literal

Raw string literals use ` + "`" + ` instead of " and do not support any escape sequences. This means that the backslash (\) can be used freely, without the need of escaping.

Since regular expressions have their own escape sequences, raw strings can improve their readability.

Before:

regexp.Compile("\\A(\\w+) profile: total \\d+\\n\\z")

After:

regexp.Compile(` + "`" + `\A(\w+) profile: total \d+\n\z` + "`" + `)

Available since
    2017.1
`

var docS1008 = `Simplify returning boolean expression

Before:

if <expr> {
  return true
}
return false

After:

return <expr>

Available since
    2017.1
`

var docS1009 = `Omit redundant nil check on slices

The len function is defined for all slices, even nil ones, which have a length of zero. It is not necessary to check if a slice is not nil before checking that its length is not zero.

Before:

if x != nil && len(x) != 0 {}

After:

if len(x) != 0 {}

Available since
    2017.1
`

var docS1010 = `Omit default slice index

When slicing, the second index defaults to the length of the value, making s[n:len(s)] and s[n:] equivalent.

Available since
    2017.1
`

var docS1011 = `Use a single append to concatenate two slices

Before:

for _, e := range y {
  x = append(x, e)
}

After:

x = append(x, y...)

Available since
    2017.1
`

var docS1012 = `Replace with time.Since(x)

The time.Since helper has the same effect as using time.Now().Sub(x) but is easier to read.

Before:

time.Now().Sub(x)

After:

time.Since(x)

Available since
    2017.1
`

var docS1016 = `Use a type conversion

Two struct types with identical fields can be converted between each other. In older versions of Go, the fields had to have identical struct tags. Since Go 1.8, however, struct tags are ignored during conversions. It is thus not necessary to manually copy every field individually.

Before:

var x T1
y := T2{
  Field1: x.Field1,
  Field2: x.Field2,
}

After:

var x T1
y := T2(x)

Available since
    2017.1
`

var docS1017 = `Replace with strings.TrimPrefix

Instead of using strings.HasPrefix and manual slicing, use the strings.TrimPrefix function. If the string doesn't start with the prefix, the original string will be returned. Using strings.TrimPrefix reduces complexity, and avoids common bugs, such as off-by-one mistakes.

Before:

if strings.HasPrefix(str, prefix) {
  str = str[len(prefix):]
}

After:

str = strings.TrimPrefix(str, prefix)

Available since
    2017.1
`

var docS1018 = `Replace with copy()

copy() permits using the same source and destination slice, even with overlapping ranges. This makes it ideal for sliding elements in a slice.

Before:

for i := 0; i < n; i++ {
  bs[i] = bs[offset+i]
}

After:

copy(bs[:n], bs[offset:])

Available since
    2017.1
`

var docS1019 = `Simplify make call

The make function has default values for the length and capacity arguments. For channels and maps, the length defaults to zero. Additionally, for slices the capacity defaults to the length.

Available since
    2017.1
`

var docS1020 = `Omit redundant nil check in type assertion

Before:

if _, ok := i.(T); ok && i != nil {}

After:

if _, ok := i.(T); ok {}

Available since
    2017.1
`

var docS1021 = `Merge variable declaration and assignment

Before:

var x uint
x = 1

After:

var x uint = 1

Available since
    2017.1
`
var docS1023 = `Omit redundant control flow

Functions that have no return value do not need a return statement as the final statement of the function.

Switches in Go do not have automatic fallthrough, unlike languages like C. It is not necessary to have a break statement as the final statement in a case block.

Available since
    2017.1
`

var docS1024 = `Replace with time.Until(x)

The time.Until helper has the same effect as using x.Sub(time.Now()) but is easier to read.

Before:

x.Sub(time.Now())

After:

time.Until(x)

Available since
    2017.1
`

var docS1025 = `Don't use fmt.Sprintf("%s", x) unnecessarily

In many instances, there are easier and more efficient ways of getting a value's string representation. Whenever a value's underlying type is a string already, or the type has a String method, they should be used directly.

Given the following shared definitions

type T1 string
type T2 int

func (T2) String() string { return "Hello, world" }

var x string
var y T1
var z T2

we can simplify the following

fmt.Sprintf("%s", x)
fmt.Sprintf("%s", y)
fmt.Sprintf("%s", z)

to

x
string(y)
z.String()

Available since
    2017.1
`

var docS1028 = `replace with fmt.Errorf

Before:

errors.New(fmt.Sprintf(...))

After:

fmt.Errorf(...)

Available since
    2017.1
`

var docS1029 = `Range over the string

Ranging over a string will yield byte offsets and runes. If the offset isn't used, this is functionally equivalent to converting the string to a slice of runes and ranging over that. Ranging directly over the string will be more performant, however, as it avoids allocating a new slice, the size of which depends on the length of the string.

Before:

for _, r := range []rune(s) {}

After:

for _, r := range s {}

Available since
    2017.1
`

var docS1030 = `Use bytes.Buffer.String or bytes.Buffer.Bytes

bytes.Buffer has both a String and a Bytes method. It is never necessary to use string(buf.Bytes()) or []byte(buf.String()) – simply use the other method.

Available since
    2017.1
`

var docS1031 = `Omit redundant nil check around loop

You can use range on nil slices and maps, the loop will simply never execute. This makes an additional nil check around the loop unnecessary.

Before:

if s != nil {
  for _, x := range s {
    ...
  }
}

After:

for _, x := range s {
  ...
}

Available since
    2017.1
`

var docS1032 = `Replace with sort.Ints(x), sort.Float64s(x), sort.Strings(x)

The sort.Ints, sort.Float64s and sort.Strings functions are easier to read than sort.Sort(sort.IntSlice(x)), sort.Sort(sort.Float64Slice(x)) and sort.Sort(sort.StringSlice(x)).

Before:

sort.Sort(sort.StringSlice(x))

After:

sort.Strings(x)

Available since
    2019.1
`
