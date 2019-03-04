[![Build Status](https://travis-ci.org/xeipuuv/gojsonschema.svg)](https://travis-ci.org/xeipuuv/gojsonschema)

# gojsonschema

## Description

An implementation of JSON Schema, based on IETF's draft v4 - Go language

References :

* http://json-schema.org
* http://json-schema.org/latest/json-schema-core.html
* http://json-schema.org/latest/json-schema-validation.html

## Installation

```
go get github.com/xeipuuv/gojsonschema
```

Dependencies :
* [github.com/xeipuuv/gojsonpointer](https://github.com/xeipuuv/gojsonpointer)
* [github.com/xeipuuv/gojsonreference](https://github.com/xeipuuv/gojsonreference)
* [github.com/stretchr/testify/assert](https://github.com/stretchr/testify#assert-package)

## Usage

### Example

```go

package main

import (
    "fmt"
    "github.com/xeipuuv/gojsonschema"
)

func main() {

    schemaLoader := gojsonschema.NewReferenceLoader("file:///home/me/schema.json")
    documentLoader := gojsonschema.NewReferenceLoader("file:///home/me/document.json")

    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        panic(err.Error())
    }

    if result.Valid() {
        fmt.Printf("The document is valid\n")
    } else {
        fmt.Printf("The document is not valid. see errors :\n")
        for _, desc := range result.Errors() {
            fmt.Printf("- %s\n", desc)
        }
    }

}


```

#### Loaders

There are various ways to load your JSON data.
In order to load your schemas and documents,
first declare an appropriate loader :

* Web / HTTP, using a reference :

```go
loader := gojsonschema.NewReferenceLoader("http://www.some_host.com/schema.json")
```

* Local file, using a reference :

```go
loader := gojsonschema.NewReferenceLoader("file:///home/me/schema.json")
```

References use the URI scheme, the prefix (file://) and a full path to the file are required.

* JSON strings :

```go
loader := gojsonschema.NewStringLoader(`{"type": "string"}`)
```

* Custom Go types :

```go
m := map[string]interface{}{"type": "string"}
loader := gojsonschema.NewGoLoader(m)
```

And

```go
type Root struct {
	Users []User `json:"users"`
}

type User struct {
	Name string `json:"name"`
}

...

data := Root{}
data.Users = append(data.Users, User{"John"})
data.Users = append(data.Users, User{"Sophia"})
data.Users = append(data.Users, User{"Bill"})

loader := gojsonschema.NewGoLoader(data)
```

#### Validation

Once the loaders are set, validation is easy :

```go
result, err := gojsonschema.Validate(schemaLoader, documentLoader)
```

Alternatively, you might want to load a schema only once and process to multiple validations :

```go
schema, err := gojsonschema.NewSchema(schemaLoader)
...
result1, err := schema.Validate(documentLoader1)
...
result2, err := schema.Validate(documentLoader2)
...
// etc ...
```

To check the result :

```go
    if result.Valid() {
    	fmt.Printf("The document is valid\n")
    } else {
        fmt.Printf("The document is not valid. see errors :\n")
        for _, err := range result.Errors() {
        	// Err implements the ResultError interface
            fmt.Printf("- %s\n", err)
        }
    }
```

## Working with Errors

The library handles string error codes which you can customize by creating your own gojsonschema.locale and setting it
```go
gojsonschema.Locale = YourCustomLocale{}
```

However, each error contains additional contextual information.

**err.Type()**: *string* Returns the "type" of error that occurred. Note you can also type check. See below

Note: An error of RequiredType has an err.Type() return value of "required"

    "required": RequiredError
    "invalid_type": InvalidTypeError
    "number_any_of": NumberAnyOfError
    "number_one_of": NumberOneOfError
    "number_all_of": NumberAllOfError
    "number_not": NumberNotError
    "missing_dependency": MissingDependencyError
    "internal": InternalError
    "enum": EnumError
    "array_no_additional_items": ArrayNoAdditionalItemsError
    "array_min_items": ArrayMinItemsError
    "array_max_items": ArrayMaxItemsError
    "unique": ItemsMustBeUniqueError
    "array_min_properties": ArrayMinPropertiesError
    "array_max_properties": ArrayMaxPropertiesError
    "additional_property_not_allowed": AdditionalPropertyNotAllowedError
    "invalid_property_pattern": InvalidPropertyPatternError
    "string_gte": StringLengthGTEError
    "string_lte": StringLengthLTEError
    "pattern": DoesNotMatchPatternError
    "multiple_of": MultipleOfError
    "number_gte": NumberGTEError
    "number_gt": NumberGTError
    "number_lte": NumberLTEError
    "number_lt": NumberLTError

**err.Value()**: *interface{}* Returns the value given

**err.Context()**: *gojsonschema.jsonContext* Returns the context. This has a String() method that will print something like this: (root).firstName

**err.Field()**: *string* Returns the fieldname in the format firstName, or for embedded properties, person.firstName. This returns the same as the String() method on *err.Context()* but removes the (root). prefix.

**err.Description()**: *string* The error description. This is based on the locale you are using. See the beginning of this section for overwriting the locale with a custom implementation.

**err.Details()**: *gojsonschema.ErrorDetails* Returns a map[string]interface{} of additional error details specific to the error. For example, GTE errors will have a "min" value, LTE will have a "max" value. See errors.go for a full description of all the error details. Every error always contains a "field" key that holds the value of *err.Field()*

Note in most cases, the err.Details() will be used to generate replacement strings in your locales. and not used directly i.e.
```
%field% must be greater than or equal to %min%
```

## Formats
JSON Schema allows for optional "format" property to validate strings against well-known formats. gojsonschema ships with all of the formats defined in the spec that you can use like this:
````json
{"type": "string", "format": "email"}
````
Available formats: date-time, hostname, email, ipv4, ipv6, uri.

For repetitive or more complex formats, you can create custom format checkers and add them to gojsonschema like this:

```go
// Define the format checker
type RoleFormatChecker struct {}

// Ensure it meets the gojsonschema.FormatChecker interface
func (f RoleFormatChecker) IsFormat(input string) bool {
    return strings.HasPrefix("ROLE_", input)
}

// Add it to the library
gojsonschema.FormatCheckers.Add("role", RoleFormatChecker{})
````

Now to use in your json schema:
````json
{"type": "string", "format": "role"}
````

## Uses

gojsonschema uses the following test suite :

https://github.com/json-schema/JSON-Schema-Test-Suite
