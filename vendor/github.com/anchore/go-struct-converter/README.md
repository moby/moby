# Go `struct` Converter

A library for converting between Go structs.

```go
chain := converter.NewChain(V1{}, V2{}, V3{})

chain.Convert(myV1struct, &myV3struct)
```

## Details

At its core, this library provides a `Convert` function, which automatically
handles converting fields with the same name, and "convertable"
types. Some examples are:
* `string` -> `string`
* `string` -> `*string`
* `int` -> `string`
* `string` -> `[]string`

The automatic conversions are implemented when there is an obvious way
to convert between the types. A lot more automatic conversions happen
-- see [the converter tests](converter_test.go) for a more comprehensive
list of what is currently supported.

Not everything can be handled automatically, however, so there is also
a `ConvertFrom` interface any struct in the graph can implement to
perform custom conversion, similar to how the stdlib `MarshalJSON` and
`UnmarshalJSON` would be implemented.

Additionally, and maybe most importantly, there is a `converter.Chain` available,
which orchestrates conversions between _multiple versions_ of structs. This could
be thought of similar to database migrations: given a starting struct and a target
struct, the `chain.Convert` function iterates through every intermediary migration
in order to arrive at the target struct.

## Basic Usage

To illustrate usage we'll start with a few basic structs, some of which
implement the `ConvertFrom` interface due to breaking changes:

```go
// --------- V1 struct definition below ---------

type V1 struct {
  Name     string
  OldField string
}

// --------- V2 struct definition below ---------

type V2 struct {
  Name     string
  NewField string // this was a renamed field
}

func (to *V2) ConvertFrom(from interface{}) error {
  if from, ok := from.(V1); ok { // forward migration
    to.NewField = from.OldField
  }
  return nil
}

// --------- V3 struct definition below ---------

type V3 struct {
  Name       []string
  FinalField []string // this field was renamed and the type was changed
}

func (to *V3) ConvertFrom(from interface{}) error {
  if from, ok := from.(V2); ok { // forward migration
    to.FinalField = []string{from.NewField}
  }
  return nil
}
```

Given these type definitions, we can easily set up a conversion chain
like this:

```go
chain := converter.NewChain(V1{}, V2{}, V3{})
```

This chain can then be used to convert from an _older version_ to a _newer 
version_. This is because our `ConvertFrom` definitions are only handling
_forward_ migrations.

This chain can be used to convert from a `V1` struct to a `V3` struct easily,
like this:

```go
v1 := // somehow get a populated v1 struct
v3 := V3{}
chain.Convert(v1, &v3)
```

Since we've defined our chain as `V1` &rarr; `V2` &rarr; `V3`, the chain will execute
conversions to all intermediary structs (`V2`, in this case) and ultimately end
when we've populated the `v3` instance.

Note we haven't needed to define any conversions on the `Name` field of any structs
since this one is convertible between structs: `string` &rarr; `string` &rarr; `[]string`.

## Backwards Migrations

If we wanted to _also_ provide backwards migrations, we could also easily add a case
to the `ConvertFrom` methods. The whole set of structs would look something like this:


```go
// --------- V1 struct definition below ---------

type V1 struct {
  Name     string
  OldField string
}

func (to *V1) ConvertFrom(from interface{}) error {
  if from, ok := from.(V2); ok { // backward migration
    to.OldField = from.NewField
  }
  return nil
}

// --------- V2 struct definition below ---------

type V2 struct {
  Name     string
  NewField string
}

func (to *V2) ConvertFrom(from interface{}) error {
  if from, ok := from.(V1); ok { // forward migration
    to.NewField = from.OldField
  }
  if from, ok := from.(V3); ok { // backward migration
    to.NewField = from.FinalField[0]
  }
  return nil
}

// --------- V3 struct definition below ---------

type V3 struct {
  Name       []string
  FinalField []string
}

func (to *V3) ConvertFrom(from interface{}) error {
  if from, ok := from.(V2); ok { // forward migration
    to.FinalField = []string{from.NewField}
  }
  return nil
}
```

At this point we could convert in either direction, for example a 
`V3` struct could convert to a `V1` struct, with the caveat that there
may be data loss, as might need to happen due to changes in the data shapes.

## Contributing

If you would like to contribute to this repository, please see the
[CONTRIBUTING.md](CONTRIBUTING.md).
