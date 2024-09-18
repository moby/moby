/*
 * ZGrab Copyright 2015 Regents of the University of Michigan
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

package json

// IANA-assigned curve ID values, see
// http://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-8
const (
	Sect163k1       TLSCurveID = 1
	Sect163r1       TLSCurveID = 2
	Sect163r2       TLSCurveID = 3
	Sect193r1       TLSCurveID = 4
	Sect193r2       TLSCurveID = 5
	Sect233k1       TLSCurveID = 6
	Sect233r1       TLSCurveID = 7
	Sect239k1       TLSCurveID = 8
	Sect283k1       TLSCurveID = 9
	Sect283r1       TLSCurveID = 10
	Sect409k1       TLSCurveID = 11
	Sect409r1       TLSCurveID = 12
	Sect571k1       TLSCurveID = 13
	Sect571r1       TLSCurveID = 14
	Secp160k1       TLSCurveID = 15
	Secp160r1       TLSCurveID = 16
	Secp160r2       TLSCurveID = 17
	Secp192k1       TLSCurveID = 18
	Secp192r1       TLSCurveID = 19
	Secp224k1       TLSCurveID = 20
	Secp224r1       TLSCurveID = 21
	Secp256k1       TLSCurveID = 22
	Secp256r1       TLSCurveID = 23
	Secp384r1       TLSCurveID = 24
	Secp521r1       TLSCurveID = 25
	BrainpoolP256r1 TLSCurveID = 26
	BrainpoolP384r1 TLSCurveID = 27
	BrainpoolP512r1 TLSCurveID = 28
)

var ecIDToName map[TLSCurveID]string
var ecNameToID map[string]TLSCurveID

func init() {
	ecIDToName = make(map[TLSCurveID]string, 64)
	ecIDToName[Sect163k1] = "sect163k1"
	ecIDToName[Sect163r1] = "sect163r1"
	ecIDToName[Sect163r2] = "sect163r2"
	ecIDToName[Sect193r1] = "sect193r1"
	ecIDToName[Sect193r2] = "sect193r2"
	ecIDToName[Sect233k1] = "sect233k1"
	ecIDToName[Sect233r1] = "sect233r1"
	ecIDToName[Sect239k1] = "sect239k1"
	ecIDToName[Sect283k1] = "sect283k1"
	ecIDToName[Sect283r1] = "sect283r1"
	ecIDToName[Sect409k1] = "sect409k1"
	ecIDToName[Sect409r1] = "sect409r1"
	ecIDToName[Sect571k1] = "sect571k1"
	ecIDToName[Sect571r1] = "sect571r1"
	ecIDToName[Secp160k1] = "secp160k1"
	ecIDToName[Secp160r1] = "secp160r1"
	ecIDToName[Secp160r2] = "secp160r2"
	ecIDToName[Secp192k1] = "secp192k1"
	ecIDToName[Secp192r1] = "secp192r1"
	ecIDToName[Secp224k1] = "secp224k1"
	ecIDToName[Secp224r1] = "secp224r1"
	ecIDToName[Secp256k1] = "secp256k1"
	ecIDToName[Secp256r1] = "secp256r1"
	ecIDToName[Secp384r1] = "secp384r1"
	ecIDToName[Secp521r1] = "secp521r1"
	ecIDToName[BrainpoolP256r1] = "brainpoolp256r1"
	ecIDToName[BrainpoolP384r1] = "brainpoolp384r1"
	ecIDToName[BrainpoolP512r1] = "brainpoolp512r1"

	ecNameToID = make(map[string]TLSCurveID, 64)
	ecNameToID["sect163k1"] = Sect163k1
	ecNameToID["sect163r1"] = Sect163r1
	ecNameToID["sect163r2"] = Sect163r2
	ecNameToID["sect193r1"] = Sect193r1
	ecNameToID["sect193r2"] = Sect193r2
	ecNameToID["sect233k1"] = Sect233k1
	ecNameToID["sect233r1"] = Sect233r1
	ecNameToID["sect239k1"] = Sect239k1
	ecNameToID["sect283k1"] = Sect283k1
	ecNameToID["sect283r1"] = Sect283r1
	ecNameToID["sect409k1"] = Sect409k1
	ecNameToID["sect409r1"] = Sect409r1
	ecNameToID["sect571k1"] = Sect571k1
	ecNameToID["sect571r1"] = Sect571r1
	ecNameToID["secp160k1"] = Secp160k1
	ecNameToID["secp160r1"] = Secp160r1
	ecNameToID["secp160r2"] = Secp160r2
	ecNameToID["secp192k1"] = Secp192k1
	ecNameToID["secp192r1"] = Secp192r1
	ecNameToID["secp224k1"] = Secp224k1
	ecNameToID["secp224r1"] = Secp224r1
	ecNameToID["secp256k1"] = Secp256k1
	ecNameToID["secp256r1"] = Secp256r1
	ecNameToID["secp384r1"] = Secp384r1
	ecNameToID["secp521r1"] = Secp521r1
	ecNameToID["brainpoolp256r1"] = BrainpoolP256r1
	ecNameToID["brainpoolp384r1"] = BrainpoolP384r1
	ecNameToID["brainpoolp512r1"] = BrainpoolP512r1
}
