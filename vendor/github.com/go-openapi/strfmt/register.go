// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

// Default is the default formats registry.
//
// NOTE: format "duration" is wire by default onto "duration-human".
var Default Registry //nolint:gochecknoglobals // package-level default registry, by design

// JSONSchema2020Registry is the format registry with JSONSchema draft 2020 formats (e.g. duration is an iso8601 duration).
var JSONSchema2020Registry Registry //nolint:gochecknoglobals // package-level default registry, by design

func init() { //nolint:gochecknoinits // registers all default string formats in the registry
	// register formats in the default registry:
	//   - byte
	//   - creditcard
	//   - email
	//   - hexcolor
	//   - hostname
	//   - ipv4
	//   - ipv6
	//   - cidr
	//   - isbn
	//   - isbn10
	//   - isbn13
	//   - mac
	//   - password
	//   - rgbcolor
	//   - ssn
	//   - uri
	//   - uuid
	//   - uuid3
	//   - uuid4
	//   - uuid5
	//   - uuid7
	//   - ulid
	//   - date, date-time
	//   - duration, duration-human, duration-iso8601
	//   - objectid
	//   - currency, country
	Default = NewSeededFormats(nil, nil)

	u := URI("")
	Default.Add("uri", &u, isRequestURI)

	eml := Email("")
	Default.Add("email", &eml, IsEmail)

	hn := Hostname("")
	Default.Add("hostname", &hn, IsHostname)

	ip4 := IPv4("")
	Default.Add("ipv4", &ip4, isIPv4)

	ip6 := IPv6("")
	Default.Add("ipv6", &ip6, isIPv6)

	cidr := CIDR("")
	Default.Add("cidr", &cidr, isCIDR)

	mac := MAC("")
	Default.Add("mac", &mac, isMAC)

	uid := UUID("")
	Default.Add("uuid", &uid, IsUUID)

	uid3 := UUID3("")
	Default.Add("uuid3", &uid3, IsUUID3)

	uid4 := UUID4("")
	Default.Add("uuid4", &uid4, IsUUID4)

	uid5 := UUID5("")
	Default.Add("uuid5", &uid5, IsUUID5)

	uid7 := UUID7("")
	Default.Add("uuid7", &uid7, IsUUID7)

	isbn := ISBN("")
	Default.Add("isbn", &isbn, func(str string) bool { return isISBN10(str) || isISBN13(str) })

	isbn10 := ISBN10("")
	Default.Add("isbn10", &isbn10, isISBN10)

	isbn13 := ISBN13("")
	Default.Add("isbn13", &isbn13, isISBN13)

	cc := CreditCard("")
	Default.Add("creditcard", &cc, isCreditCard)

	ssn := SSN("")
	Default.Add("ssn", &ssn, isSSN)

	hc := HexColor("")
	Default.Add("hexcolor", &hc, isHexcolor)

	rc := RGBColor("")
	Default.Add("rgbcolor", &rc, isRGBcolor)

	b64 := Base64([]byte(nil))
	Default.Add("byte", &b64, isBase64)

	pw := Password("")
	Default.Add("password", &pw, func(_ string) bool { return true })

	d := Date{}
	Default.Add("date", &d, IsDate)

	dt := DateTime{}
	Default.Add("datetime", &dt, IsDateTime)

	du := Duration(0)
	Default.Add("duration", &du, IsDuration)
	Default.Add("duration-human", &du, IsDuration)

	di := DurationISO8601(0)
	Default.Add("duration-iso8601", &di, IsDurationISO8601)

	var id ObjectId
	Default.Add("bsonobjectid", &id, IsBSONObjectID)

	ulid := ULID{}
	Default.Add("ulid", &ulid, IsULID)

	cur := Currency{}
	Default.Add("currency", &cur, IsCurrency)

	co := Country{}
	Default.Add("country", &co, IsCountry)

	def, ok := Default.(*defaultFormats)
	if !ok {
		panic("internal error: can't initialize")
	}

	JSONSchema2020Registry = NewSeededFormats(def.data, JSONSchema2020Normalizer)
}
