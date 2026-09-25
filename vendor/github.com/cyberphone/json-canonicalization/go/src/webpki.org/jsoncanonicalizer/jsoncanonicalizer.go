//
//  Copyright 2006-2019 WebPKI.org (http://webpki.org).
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//
 
// This package transforms JSON data in UTF-8 according to:
// https://tools.ietf.org/html/draft-rundgren-json-canonicalization-scheme-02

package jsoncanonicalizer

import (
    "errors"
    "container/list"
    "fmt"
    "strconv"
    "strings"
    "unicode/utf16"
)

type nameValueType struct {
    name string
    sortKey []uint16
    value string
}

// JSON standard escapes (modulo \u)
var asciiEscapes  = []byte{'\\', '"', 'b',  'f',  'n',  'r',  't'}
var binaryEscapes = []byte{'\\', '"', '\b', '\f', '\n', '\r', '\t'}

// JSON literals
var literals      = []string{"true", "false", "null"}
    
func Transform(jsonData []byte) (result []byte, e error) {

    // JSON data MUST be UTF-8 encoded
    var jsonDataLength int = len(jsonData)

    // Current pointer in jsonData
    var index int = 0

    // "Forward" declarations are needed for closures referring each other
    var parseElement func() string
    var parseSimpleType func() string
    var parseQuotedString func() string
    var parseObject func() string
    var parseArray func() string

    var globalError error = nil

    checkError := func(e error) {
        // We only honor the first reported error
        if globalError == nil {
            globalError = e
        }
    }
    
    setError := func(msg string) {
        checkError(errors.New(msg))
    }

    isWhiteSpace := func(c byte) bool {
        return c == 0x20 || c == 0x0a || c == 0x0d || c == 0x09
    }

    nextChar := func() byte {
        if index < jsonDataLength {
            c := jsonData[index]
            if c > 0x7f {
                setError("Unexpected non-ASCII character")
            }
            index++
            return c
        }
        setError("Unexpected EOF reached")
        return '"'
    }

    scan := func() byte {
        for {
            c := nextChar()
            if isWhiteSpace(c) {
                continue;
            }
            return c
        }
    }

    scanFor := func(expected byte) {
        c := scan()
        if c != expected {
            setError("Expected '" + string(expected) + "' but got '" + string(c) + "'")
        }
    }

    getUEscape := func() rune {
        start := index
        nextChar()
        nextChar()
        nextChar()
        nextChar()
        if globalError != nil {
            return 0
        }
        u16, err := strconv.ParseUint(string(jsonData[start:index]), 16, 64)
        checkError(err)
        return rune(u16)
    }

    testNextNonWhiteSpaceChar := func() byte {
        save := index
        c := scan()
        index = save
        return c
    }

    decorateString := func(rawUTF8 string) string {
        var quotedString strings.Builder
        quotedString.WriteByte('"')
      CoreLoop:
        for _, c := range []byte(rawUTF8) {
            // Is this within the JSON standard escapes?
            for i, esc := range binaryEscapes {
                if esc == c {
                    quotedString.WriteByte('\\')
                    quotedString.WriteByte(asciiEscapes[i])
                    continue CoreLoop
                }
            }
            if c < 0x20 {
                // Other ASCII control characters must be escaped with \uhhhh
                quotedString.WriteString(fmt.Sprintf("\\u%04x", c))         
            } else {
                quotedString.WriteByte(c)
            }
        }
        quotedString.WriteByte('"')
        return quotedString.String()
    }

    parseQuotedString = func() string {
        var rawString strings.Builder
      CoreLoop:
        for globalError == nil {
            var c byte
            if index < jsonDataLength {
                c = jsonData[index]
                index++
            } else {
                nextChar()
                break
            }
            if (c == '"') {
                break;
            }
            if c < ' ' {
                setError("Unterminated string literal")
            } else if c == '\\' {
                // Escape sequence
                c = nextChar()
                if c == 'u' {
                    // The \u escape
                    firstUTF16 := getUEscape()
                    if utf16.IsSurrogate(firstUTF16) {
                        // If the first UTF-16 code unit has a certain value there must be
                        // another succeeding UTF-16 code unit as well
                        if nextChar() != '\\' || nextChar() != 'u' {
                            setError("Missing surrogate")
                        } else {
                            // Output the UTF-32 code point as UTF-8
                            rawString.WriteRune(utf16.DecodeRune(firstUTF16, getUEscape()))
                        }
                    } else {
                        // Single UTF-16 code identical to UTF-32.  Output as UTF-8
                        rawString.WriteRune(firstUTF16)
                    }
                } else if c == '/' {
                    // Benign but useless escape
                    rawString.WriteByte('/')
                } else {
                    // The JSON standard escapes
                    for i, esc := range asciiEscapes {
                        if esc == c {
                            rawString.WriteByte(binaryEscapes[i])
                            continue CoreLoop
                        }
                    }
                    setError("Unexpected escape: \\" + string(c))
                }
            } else {
                // Just an ordinary ASCII character alternatively a UTF-8 byte
                // outside of ASCII.
                // Note that properly formatted UTF-8 never clashes with ASCII
                // making byte per byte search for ASCII break characters work
                // as expected.
                rawString.WriteByte(c)
            }
        }
        return rawString.String()
    }

    parseSimpleType = func() string {
        var token strings.Builder
        index--
        for globalError == nil {
            c := testNextNonWhiteSpaceChar()
            if c == ',' || c == ']' || c == '}' {
                break;
            }
            c = nextChar()
            if isWhiteSpace(c) {
                break
            }
            token.WriteByte(c)
        }
        if token.Len() == 0 {
            setError("Missing argument")
        }
        value := token.String()
        // Is it a JSON literal?
        for _, literal := range literals {
            if literal == value {
                return literal
            }
        }
        // Apparently not so we assume that it is a I-JSON number
        ieeeF64, err := strconv.ParseFloat(value, 64)
        checkError(err)
        value, err = NumberToJSON(ieeeF64)
        checkError(err)
        return value
    }

    parseElement = func() string {
        switch scan() {
            case '{':
                return parseObject()
            case '"':
                return decorateString(parseQuotedString())
            case '[':
                return parseArray()
            default:
                return parseSimpleType()
        }
    }

    parseArray = func() string {
        var arrayData strings.Builder
        arrayData.WriteByte('[')
        var next bool = false
        for globalError == nil && testNextNonWhiteSpaceChar() != ']' {
            if next {
                scanFor(',')
                arrayData.WriteByte(',')
            } else {
                next = true
            }
            arrayData.WriteString(parseElement())
        }
        scan()
        arrayData.WriteByte(']')
        return arrayData.String()
    }

    lexicographicallyPrecedes := func(sortKey []uint16, e *list.Element) bool {
        // Find the minimum length of the sortKeys
        oldSortKey := e.Value.(nameValueType).sortKey
        minLength := len(oldSortKey)
        if minLength > len(sortKey) {
            minLength = len(sortKey)
        }
        for q := 0; q < minLength; q++ {
            diff := int(sortKey[q]) - int(oldSortKey[q])
            if diff < 0 {
                // Smaller => Precedes
                return true
            } else if diff > 0 {
                // Bigger => No match
                return false
            }
            // Still equal => Continue
        }
        // The sortKeys compared equal up to minLength
        if len(sortKey) < len(oldSortKey) {
            // Shorter => Precedes
            return true
        }
        if len(sortKey) == len(oldSortKey) {
            setError("Duplicate key: " + e.Value.(nameValueType).name)
        }
        // Longer => No match
        return false
    }

    parseObject = func() string {
        nameValueList := list.New()
        var next bool = false
      CoreLoop:
        for globalError == nil && testNextNonWhiteSpaceChar() != '}' {
            if next {
                scanFor(',')
            }
            next = true
            scanFor('"')
            rawUTF8 := parseQuotedString()
            if globalError != nil {
                break;
            }
            // Sort keys on UTF-16 code units
            // Since UTF-8 doesn't have endianess this is just a value transformation
            // In the Go case the transformation is UTF-8 => UTF-32 => UTF-16
            sortKey := utf16.Encode([]rune(rawUTF8))
            scanFor(':')
            nameValue := nameValueType{rawUTF8, sortKey, parseElement()}
            for e := nameValueList.Front(); e != nil; e = e.Next() {
                // Check if the key is smaller than a previous key
                if lexicographicallyPrecedes(sortKey, e) {
                    // Precedes => Insert before and exit sorting
                    nameValueList.InsertBefore(nameValue, e)
                    continue CoreLoop
                }
                // Continue searching for a possibly succeeding sortKey
                // (which is straightforward since the list is ordered)
            }
            // The sortKey is either the first or is succeeding all previous sortKeys
            nameValueList.PushBack(nameValue)
        }
        // Scan away '}'
        scan()
        // Now everything is sorted so we can properly serialize the object
        var objectData strings.Builder
        objectData.WriteByte('{')
        next = false
        for e := nameValueList.Front(); e != nil; e = e.Next() {
            if next {
                objectData.WriteByte(',')
            }
            next = true
            nameValue := e.Value.(nameValueType)
            objectData.WriteString(decorateString(nameValue.name))
            objectData.WriteByte(':')
            objectData.WriteString(nameValue.value)
        }
        objectData.WriteByte('}')
        return objectData.String()
    }

    /////////////////////////////////////////////////
    // This is where Transform actually begins...  //
    /////////////////////////////////////////////////
    var transformed string

    if testNextNonWhiteSpaceChar() == '[' {
        scan()
        transformed = parseArray()
    } else {
        scanFor('{')
        transformed = parseObject()
    }
    for index < jsonDataLength {
        if !isWhiteSpace(jsonData[index]) {
            setError("Improperly terminated JSON object")
            break;
        }
        index++
    }
    return []byte(transformed), globalError
}