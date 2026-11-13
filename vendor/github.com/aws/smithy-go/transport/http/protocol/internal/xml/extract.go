package xml

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// ExtractElement finds the first occurrence of the named element in the XML
// and returns it as raw bytes, including the element's opening and closing
// tags.
//
// This is used to extract both success and error responses from the protocol
// XML that wraps them.
//
// For example for awsquery, with an operation output shape named
// XmlListsResult:
//
//	<XmlListsResponse>
//	 	<XmlListsResult>
//	 		<member1>foo</member1>
//	 		...
//	 	</XmlListsResult>
//	</XmlListsResponse>
//
// ExtractElement(payload, "XmlListsResult") yields:
//
//	<XmlListsResult><member1>foo</member1>...</XmlListsResult>
//
// ExtractElement will forward the io.EOF returned by the underlying decoder
// if the target element is not found, which the caller can index on if
// they're looking for an optional element.
func ExtractElement(payload []byte, name string) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(payload))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, name) {
			continue
		}

		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		if err := enc.EncodeToken(start); err != nil {
			return nil, err
		}
		if err := enc.Flush(); err != nil {
			return nil, err
		}

		depth := 1
		for depth > 0 {
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if err := enc.EncodeToken(tok); err != nil {
				return nil, err
			}
			switch tok.(type) {
			case xml.StartElement:
				depth++
			case xml.EndElement:
				depth--
			}
		}

		if err := enc.Flush(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
}
