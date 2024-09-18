/*
 * ZLint Copyright 2021 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package util

import "strings"

var countries = map[string]bool{
	"AD": true, "AE": true, "AF": true, "AG": true, "AI": true, "AL": true, "AM": true, "AN": true, "AO": true, "AQ": true, "AR": true,
	"AS": true, "AT": true, "AU": true, "AW": true, "AX": true, "AZ": true, "BA": true, "BB": true, "BD": true, "BE": true, "BF": true, "BG": true,
	"BH": true, "BI": true, "BJ": true, "BL": true, "BM": true, "BN": true, "BO": true, "BQ": true, "BR": true, "BS": true, "BT": true, "BV": true,
	"BW": true, "BY": true, "BZ": true, "CA": true, "CC": true, "CD": true, "CF": true, "CG": true, "CH": true, "CI": true, "CK": true, "CL": true,
	"CM": true, "CN": true, "CO": true, "CR": true, "CU": true, "CV": true, "CW": true, "CX": true, "CY": true, "CZ": true, "DE": true, "DJ": true,
	"DK": true, "DM": true, "DO": true, "DZ": true, "EC": true, "EE": true, "EG": true, "EH": true, "ER": true, "ES": true, "ET": true, "FI": true,
	"FJ": true, "FK": true, "FM": true, "FO": true, "FR": true, "GA": true, "GB": true, "GD": true, "GE": true, "GF": true, "GG": true, "GH": true,
	"GI": true, "GL": true, "GM": true, "GN": true, "GP": true, "GQ": true, "GR": true, "GS": true, "GT": true, "GU": true, "GW": true, "GY": true,
	"HK": true, "HM": true, "HN": true, "HR": true, "HT": true, "HU": true, "ID": true, "IE": true, "IL": true, "IM": true, "IN": true, "IO": true,
	"IQ": true, "IR": true, "IS": true, "IT": true, "JE": true, "JM": true, "JO": true, "JP": true, "KE": true, "KG": true, "KH": true, "KI": true,
	"KM": true, "KN": true, "KP": true, "KR": true, "KW": true, "KY": true, "KZ": true, "LA": true, "LB": true, "LC": true, "LI": true, "LK": true,
	"LR": true, "LS": true, "LT": true, "LU": true, "LV": true, "LY": true, "MA": true, "MC": true, "MD": true, "ME": true, "MF": true, "MG": true,
	"MH": true, "MK": true, "ML": true, "MM": true, "MN": true, "MO": true, "MP": true, "MQ": true, "MR": true, "MS": true, "MT": true, "MU": true,
	"MV": true, "MW": true, "MX": true, "MY": true, "MZ": true, "NA": true, "NC": true, "NE": true, "NF": true, "NG": true, "NI": true, "NL": true,
	"NO": true, "NP": true, "NR": true, "NU": true, "NZ": true, "OM": true, "PA": true, "PE": true, "PF": true, "PG": true, "PH": true, "PK": true,
	"PL": true, "PM": true, "PN": true, "PR": true, "PS": true, "PT": true, "PW": true, "PY": true, "QA": true, "RE": true, "RO": true, "RS": true,
	"RU": true, "RW": true, "SA": true, "SB": true, "SC": true, "SD": true, "SE": true, "SG": true, "SH": true, "SI": true, "SJ": true, "SK": true,
	"SL": true, "SM": true, "SN": true, "SO": true, "SR": true, "SS": true, "ST": true, "SV": true, "SX": true, "SY": true, "SZ": true, "TC": true,
	"TD": true, "TF": true, "TG": true, "TH": true, "TJ": true, "TK": true, "TL": true, "TM": true, "TN": true, "TO": true, "TR": true, "TT": true,
	"TV": true, "TW": true, "TZ": true, "UA": true, "UG": true, "UM": true, "US": true, "UY": true, "UZ": true, "VA": true, "VC": true, "VE": true,
	"VG": true, "VI": true, "VN": true, "VU": true, "WF": true, "WS": true, "YE": true, "YT": true, "ZA": true, "ZM": true, "ZW": true, "XX": true,
}

// IsISOCountryCode returns true if the input is a known two-letter country
// code.
//
// TODO: Document where the list of known countries came from.
func IsISOCountryCode(in string) bool {
	in = strings.ToUpper(in)
	_, ok := countries[in]
	return ok
}
