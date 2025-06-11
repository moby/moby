// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package v2_3

import (
	"encoding/json"
	"strings"

	"github.com/spdx/tools-golang/json/marshal"
	"github.com/spdx/tools-golang/spdx/v2/common"
)

// Package is a Package section of an SPDX Document
type Package struct {
	// NOT PART OF SPEC
	// flag: does this "package" contain files that were in fact "unpackaged",
	// e.g. included directly in the Document without being in a Package?
	IsUnpackaged bool `json:"-" yaml:"-"`

	// 7.1: Package Name
	// Cardinality: mandatory, one
	PackageName string `json:"name"`

	// 7.2: Package SPDX Identifier: "SPDXRef-[idstring]"
	// Cardinality: mandatory, one
	PackageSPDXIdentifier common.ElementID `json:"SPDXID"`

	// 7.3: Package Version
	// Cardinality: optional, one
	PackageVersion string `json:"versionInfo,omitempty"`

	// 7.4: Package File Name
	// Cardinality: optional, one
	PackageFileName string `json:"packageFileName,omitempty"`

	// 7.5: Package Supplier: may have single result for either Person or Organization,
	//                        or NOASSERTION
	// Cardinality: optional, one
	PackageSupplier *common.Supplier `json:"supplier,omitempty"`

	// 7.6: Package Originator: may have single result for either Person or Organization,
	//                          or NOASSERTION
	// Cardinality: optional, one
	PackageOriginator *common.Originator `json:"originator,omitempty"`

	// 7.7: Package Download Location
	// Cardinality: mandatory, one
	PackageDownloadLocation string `json:"downloadLocation"`

	// 7.8: FilesAnalyzed
	// Cardinality: optional, one; default value is "true" if omitted
	FilesAnalyzed bool `json:"filesAnalyzed"`
	// NOT PART OF SPEC: did FilesAnalyzed tag appear?
	IsFilesAnalyzedTagPresent bool `json:"-" yaml:"-"`

	// 7.9: Package Verification Code
	// Cardinality: if FilesAnalyzed == true must be present, if FilesAnalyzed == false must be omitted
	PackageVerificationCode *common.PackageVerificationCode `json:"packageVerificationCode,omitempty"`

	// 7.10: Package Checksum: may have keys for SHA1, SHA256, SHA512, MD5, SHA3-256, SHA3-384, SHA3-512, BLAKE2b-256, BLAKE2b-384, BLAKE2b-512, BLAKE3, ADLER32
	// Cardinality: optional, one or many
	PackageChecksums []common.Checksum `json:"checksums,omitempty"`

	// 7.11: Package Home Page
	// Cardinality: optional, one
	PackageHomePage string `json:"homepage,omitempty"`

	// 7.12: Source Information
	// Cardinality: optional, one
	PackageSourceInfo string `json:"sourceInfo,omitempty"`

	// 7.13: Concluded License: SPDX License Expression, "NONE" or "NOASSERTION"
	// Cardinality: optional, one
	PackageLicenseConcluded string `json:"licenseConcluded,omitempty"`

	// 7.14: All Licenses Info from Files: SPDX License Expression, "NONE" or "NOASSERTION"
	// Cardinality: optional, one or many if filesAnalyzed is true / omitted;
	//              zero (must be omitted) if filesAnalyzed is false
	PackageLicenseInfoFromFiles []string `json:"licenseInfoFromFiles,omitempty"`

	// 7.15: Declared License: SPDX License Expression, "NONE" or "NOASSERTION"
	// Cardinality: optional, one
	PackageLicenseDeclared string `json:"licenseDeclared,omitempty"`

	// 7.16: Comments on License
	// Cardinality: optional, one
	PackageLicenseComments string `json:"licenseComments,omitempty"`

	// 7.17: Copyright Text: copyright notice(s) text, "NONE" or "NOASSERTION"
	// Cardinality: optional, zero or one
	PackageCopyrightText string `json:"copyrightText,omitempty"`

	// 7.18: Package Summary Description
	// Cardinality: optional, one
	PackageSummary string `json:"summary,omitempty"`

	// 7.19: Package Detailed Description
	// Cardinality: optional, one
	PackageDescription string `json:"description,omitempty"`

	// 7.20: Package Comment
	// Cardinality: optional, one
	PackageComment string `json:"comment,omitempty"`

	// 7.21: Package External Reference
	// Cardinality: optional, one or many
	PackageExternalReferences []*PackageExternalReference `json:"externalRefs,omitempty"`

	// 7.22: Package External Reference Comment
	// Cardinality: conditional (optional, one) for each External Reference
	// contained within PackageExternalReference struct, if present

	// 7.23: Package Attribution Text
	// Cardinality: optional, one or many
	PackageAttributionTexts []string `json:"attributionTexts,omitempty"`

	// 7.24: Primary Package Purpose
	// Cardinality: optional, one or many
	// Allowed values: APPLICATION, FRAMEWORK, LIBRARY, CONTAINER, OPERATING-SYSTEM, DEVICE, FIRMWARE, SOURCE, ARCHIVE, FILE, INSTALL, OTHER
	PrimaryPackagePurpose string `json:"primaryPackagePurpose,omitempty"`

	// 7.25: Release Date: YYYY-MM-DDThh:mm:ssZ
	// Cardinality: optional, one
	ReleaseDate string `json:"releaseDate,omitempty"`

	// 7.26: Build Date: YYYY-MM-DDThh:mm:ssZ
	// Cardinality: optional, one
	BuiltDate string `json:"builtDate,omitempty"`

	// 7.27: Valid Until Date: YYYY-MM-DDThh:mm:ssZ
	// Cardinality: optional, one
	ValidUntilDate string `json:"validUntilDate,omitempty"`

	// Files contained in this Package
	Files []*File `json:"files,omitempty"`

	Annotations []Annotation `json:"annotations,omitempty"`

	// this field is only used when decoding JSON to translate the hasFiles
	// property to relationships
	hasFiles []common.DocElementID
}

func (p *Package) UnmarshalJSON(b []byte) error {
	type pkg Package
	type extras struct {
		HasFiles      []common.DocElementID `json:"hasFiles"`
		FilesAnalyzed *bool                 `json:"filesAnalyzed"`
	}

	var p2 pkg
	if err := json.Unmarshal(b, &p2); err != nil {
		return err
	}

	var e extras
	if err := json.Unmarshal(b, &e); err != nil {
		return err
	}

	*p = Package(p2)

	p.hasFiles = e.HasFiles

	// FilesAnalyzed defaults to true if omitted
	if e.FilesAnalyzed == nil {
		p.FilesAnalyzed = true
	} else {
		p.IsFilesAnalyzedTagPresent = true
	}

	return nil
}

var _ json.Unmarshaler = (*Package)(nil)

// PackageExternalReference is an External Reference to additional info
// about a Package, as defined in section 7.21
type PackageExternalReference struct {
	// category is "SECURITY", "PACKAGE-MANAGER" or "OTHER"
	Category string `json:"referenceCategory"`

	// type is an [idstring] as defined in Appendix VI;
	// called RefType here due to "type" being a Golang keyword
	RefType string `json:"referenceType"`

	// locator is a unique string to access the package-specific
	// info, metadata or content within the target location
	Locator string `json:"referenceLocator"`

	// 7.22: Package External Reference Comment
	// Cardinality: conditional (optional, one) for each External Reference
	ExternalRefComment string `json:"comment,omitempty"`
}

var _ json.Unmarshaler = (*PackageExternalReference)(nil)

func (r *PackageExternalReference) UnmarshalJSON(b []byte) error {
	type ref PackageExternalReference
	var rr ref
	if err := json.Unmarshal(b, &rr); err != nil {
		return err
	}

	rr.Category = strings.ReplaceAll(rr.Category, "_", "-")

	*r = PackageExternalReference(rr)
	return nil
}

var _ json.Marshaler = (*PackageExternalReference)(nil)

func (r *PackageExternalReference) MarshalJSON() ([]byte, error) {
	type ref PackageExternalReference
	var rr ref

	rr = ref(*r)

	rr.Category = strings.ReplaceAll(rr.Category, "_", "-")

	return marshal.JSON(&rr)
}
