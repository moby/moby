// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkix

import (
	"encoding/asn1"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type auxAttributeTypeAndValue struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (a *AttributeTypeAndValue) MarshalJSON() ([]byte, error) {
	aux := auxAttributeTypeAndValue{}
	aux.Type = a.Type.String()
	if s, ok := a.Value.(string); ok {
		aux.Value = s
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (a *AttributeTypeAndValue) UnmarshalJSON(b []byte) error {
	aux := auxAttributeTypeAndValue{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	a.Type = nil
	if len(aux.Type) > 0 {
		parts := strings.Split(aux.Type, ".")
		for _, part := range parts {
			i, err := strconv.Atoi(part)
			if err != nil {
				return err
			}
			a.Type = append(a.Type, i)
		}
	}
	a.Value = aux.Value
	return nil
}

type auxOtherName struct {
	ID    string `json:"id,omitempty"`
	Value []byte `json:"value,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (o *OtherName) MarshalJSON() ([]byte, error) {
	aux := auxOtherName{
		ID:    o.TypeID.String(),
		Value: o.Value.Bytes,
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (o *OtherName) UnmarshalJSON(b []byte) (err error) {
	aux := auxOtherName{}
	if err = json.Unmarshal(b, &aux); err != nil {
		return
	}

	// Turn dot-notation back into an OID
	if len(aux.ID) == 0 {
		return errors.New("empty type ID")
	}
	parts := strings.Split(aux.ID, ".")
	o.TypeID = nil
	for _, part := range parts {
		i, err := strconv.Atoi(part)
		if err != nil {
			return err
		}
		o.TypeID = append(o.TypeID, i)
	}

	// Build the ASN.1 value
	o.Value = asn1.RawValue{
		Tag:        0,
		Class:      asn1.ClassContextSpecific,
		IsCompound: true,
		Bytes:      aux.Value,
	}
	o.Value.FullBytes, err = asn1.Marshal(o.Value)
	return
}

type auxExtension struct {
	ID       string `json:"id,omitempty"`
	Critical bool   `json:"critical"`
	Value    []byte `json:"value,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (ext *Extension) MarshalJSON() ([]byte, error) {
	aux := auxExtension{
		ID:       ext.Id.String(),
		Critical: ext.Critical,
		Value:    ext.Value,
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ext *Extension) UnmarshalJSON(b []byte) (err error) {
	aux := auxExtension{}
	if err = json.Unmarshal(b, &aux); err != nil {
		return
	}

	parts := strings.Split(aux.ID, ".")
	for _, part := range parts {
		i, err := strconv.Atoi(part)
		if err != nil {
			return err
		}
		ext.Id = append(ext.Id, i)
	}
	ext.Critical = aux.Critical
	ext.Value = aux.Value
	return
}

type auxName struct {
	CommonName         []string `json:"common_name,omitempty"`
	SerialNumber       []string `json:"serial_number,omitempty"`
	Country            []string `json:"country,omitempty"`
	Locality           []string `json:"locality,omitempty"`
	Province           []string `json:"province,omitempty"`
	StreetAddress      []string `json:"street_address,omitempty"`
	Organization       []string `json:"organization,omitempty"`
	OrganizationalUnit []string `json:"organizational_unit,omitempty"`
	PostalCode         []string `json:"postal_code,omitempty"`
	DomainComponent    []string `json:"domain_component,omitempty"`
	EmailAddress       []string `json:"email_address,omitempty"`
	GivenName          []string `json:"given_name,omitempty"`
	Surname            []string `json:"surname,omitempty"`
	// EV
	JurisdictionCountry  []string `json:"jurisdiction_country,omitempty"`
	JurisdictionLocality []string `json:"jurisdiction_locality,omitempty"`
	JurisdictionProvince []string `json:"jurisdiction_province,omitempty"`

	// QWACS
	OrganizationID []string `json:"organization_id,omitempty"`

	UnknownAttributes []AttributeTypeAndValue `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (n *Name) MarshalJSON() ([]byte, error) {
	aux := auxName{}
	attrs := n.ToRDNSequence()
	for _, attrSet := range attrs {
		for _, a := range attrSet {
			s, ok := a.Value.(string)
			if !ok {
				continue
			}
			if a.Type.Equal(oidCommonName) {
				aux.CommonName = append(aux.CommonName, s)
			} else if a.Type.Equal(oidSurname) {
				aux.Surname = append(aux.Surname, s)
			} else if a.Type.Equal(oidSerialNumber) {
				aux.SerialNumber = append(aux.SerialNumber, s)
			} else if a.Type.Equal(oidCountry) {
				aux.Country = append(aux.Country, s)
			} else if a.Type.Equal(oidLocality) {
				aux.Locality = append(aux.Locality, s)
			} else if a.Type.Equal(oidProvince) {
				aux.Province = append(aux.Province, s)
			} else if a.Type.Equal(oidStreetAddress) {
				aux.StreetAddress = append(aux.StreetAddress, s)
			} else if a.Type.Equal(oidOrganization) {
				aux.Organization = append(aux.Organization, s)
			} else if a.Type.Equal(oidGivenName) {
				aux.GivenName = append(aux.GivenName, s)
			} else if a.Type.Equal(oidOrganizationalUnit) {
				aux.OrganizationalUnit = append(aux.OrganizationalUnit, s)
			} else if a.Type.Equal(oidPostalCode) {
				aux.PostalCode = append(aux.PostalCode, s)
			} else if a.Type.Equal(oidDomainComponent) {
				aux.DomainComponent = append(aux.DomainComponent, s)
			} else if a.Type.Equal(oidDNEmailAddress) {
				aux.EmailAddress = append(aux.EmailAddress, s)
				// EV
			} else if a.Type.Equal(oidJurisdictionCountry) {
				aux.JurisdictionCountry = append(aux.JurisdictionCountry, s)
			} else if a.Type.Equal(oidJurisdictionLocality) {
				aux.JurisdictionLocality = append(aux.JurisdictionLocality, s)
			} else if a.Type.Equal(oidJurisdictionProvince) {
				aux.JurisdictionProvince = append(aux.JurisdictionProvince, s)
			} else if a.Type.Equal(oidOrganizationID) {
				aux.OrganizationID = append(aux.OrganizationID, s)
			} else {
				aux.UnknownAttributes = append(aux.UnknownAttributes, a)
			}
		}
	}
	return json.Marshal(&aux)
}

func appendATV(names []AttributeTypeAndValue, fieldVals []string, asn1Id asn1.ObjectIdentifier) []AttributeTypeAndValue {
	if len(fieldVals) == 0 {
		return names
	}

	for _, val := range fieldVals {
		names = append(names, AttributeTypeAndValue{Type: asn1Id, Value: val})
	}

	return names
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (n *Name) UnmarshalJSON(b []byte) error {
	aux := auxName{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	// Populate Names as []AttributeTypeAndValue
	n.Names = appendATV(n.Names, aux.Country, oidCountry)
	n.Names = appendATV(n.Names, aux.Organization, oidOrganization)
	n.Names = appendATV(n.Names, aux.OrganizationalUnit, oidOrganizationalUnit)
	n.Names = appendATV(n.Names, aux.Locality, oidLocality)
	n.Names = appendATV(n.Names, aux.Province, oidProvince)
	n.Names = appendATV(n.Names, aux.StreetAddress, oidStreetAddress)
	n.Names = appendATV(n.Names, aux.PostalCode, oidPostalCode)
	n.Names = appendATV(n.Names, aux.DomainComponent, oidDomainComponent)
	n.Names = appendATV(n.Names, aux.EmailAddress, oidDNEmailAddress)
	// EV
	n.Names = appendATV(n.Names, aux.JurisdictionCountry, oidJurisdictionCountry)
	n.Names = appendATV(n.Names, aux.JurisdictionLocality, oidJurisdictionLocality)
	n.Names = appendATV(n.Names, aux.JurisdictionProvince, oidJurisdictionProvince)

	n.Names = appendATV(n.Names, aux.CommonName, oidCommonName)
	n.Names = appendATV(n.Names, aux.SerialNumber, oidSerialNumber)

	// Populate specific fields as []string
	n.Country = aux.Country
	n.Organization = aux.Organization
	n.OrganizationalUnit = aux.OrganizationalUnit
	n.Locality = aux.Locality
	n.Province = aux.Province
	n.StreetAddress = aux.StreetAddress
	n.PostalCode = aux.PostalCode
	n.DomainComponent = aux.DomainComponent
	// EV
	n.JurisdictionCountry = aux.JurisdictionCountry
	n.JurisdictionLocality = aux.JurisdictionLocality
	n.JurisdictionProvince = aux.JurisdictionProvince

	// CommonName and SerialNumber are not arrays.
	if len(aux.CommonName) > 0 {
		n.CommonName = aux.CommonName[0]
	}
	if len(aux.SerialNumber) > 0 {
		n.SerialNumber = aux.SerialNumber[0]
	}

	// Add "extra" commonNames and serialNumbers to ExtraNames.
	if len(aux.CommonName) > 1 {
		n.ExtraNames = appendATV(n.ExtraNames, aux.CommonName[1:], oidCommonName)
	}
	if len(aux.SerialNumber) > 1 {
		n.ExtraNames = appendATV(n.ExtraNames, aux.SerialNumber[1:], oidSerialNumber)
	}

	return nil
}
