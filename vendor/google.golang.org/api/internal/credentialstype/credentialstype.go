// Copyright 2024 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package credentialstype defines the CredType used for specifying the type of JSON credentials.
package credentialstype

import (
	"encoding/json"
	"fmt"
	"slices"
)

// CredType specifies the type of JSON credentials.
type CredType string

const (
	// Unknown represents an unknown JSON file type.
	Unknown CredType = ""
	// ServiceAccount represents a service account file type.
	ServiceAccount CredType = "service_account"
	// AuthorizedUser represents an authorized user credentials file type.
	AuthorizedUser CredType = "authorized_user"
	// ImpersonatedServiceAccount represents an impersonated service account file type.
	//
	// IMPORTANT:
	// This credential type does not validate the credential configuration. A security
	// risk occurs when a credential configuration configured with malicious urls
	// is used.
	// You should validate credential configurations provided by untrusted sources.
	// See [Security requirements when using credential configurations from an external
	// source] https://cloud.google.com/docs/authentication/external/externally-sourced-credentials
	// for more details.
	ImpersonatedServiceAccount CredType = "impersonated_service_account"
	// ExternalAccount represents an external account file type.
	//
	// IMPORTANT:
	// This credential type does not validate the credential configuration. A security
	// risk occurs when a credential configuration configured with malicious urls
	// is used.
	// You should validate credential configurations provided by untrusted sources.
	// See [Security requirements when using credential configurations from an external
	// source] https://cloud.google.com/docs/authentication/external/externally-sourced-credentials
	// for more details.
	ExternalAccount CredType = "external_account"
	// GDCHServiceAccount represents a GDCH service account file type.
	GDCHServiceAccount CredType = "gdc_service_account"
	// ExternalAccountAuthorizedUser represents an external account authorized user file type.
	ExternalAccountAuthorizedUser CredType = "external_account_authorized_user"
)

var knownTypes = map[CredType]bool{
	ServiceAccount:                true,
	AuthorizedUser:                true,
	ImpersonatedServiceAccount:    true,
	ExternalAccount:               true,
	GDCHServiceAccount:            true,
	ExternalAccountAuthorizedUser: true,
}

// GetCredType returns the credentials type or the Unknown type,
// or an error for empty data or failure to unmarshal JSON.
func GetCredType(data []byte) (CredType, error) {
	var t CredType
	if len(data) == 0 {
		return t, fmt.Errorf("credential provided is 0 bytes")
	}
	var f struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return t, err
	}
	t = parseCredType(f.Type)
	return t, nil
}

// CheckCredentialType checks if the provided JSON bytes match the expected
// credential type and, if present, one of the allowed credential types.
// An error is returned if the JSON is invalid, the type field is missing,
// or the types do not match expected and (if present) allowed.
func CheckCredentialType(b []byte, expected CredType, allowed ...CredType) error {
	var f struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("unable to parse credential type: %w", err)
	}
	if f.Type == "" {
		return fmt.Errorf("missing `type` field in credential")
	}
	credType := CredType(f.Type)
	if credType != expected {
		return fmt.Errorf("credential type mismatch: got %q, expected %q", credType, expected)
	}
	if len(allowed) == 0 {
		return nil
	}
	if !slices.Contains(allowed, credType) {
		return fmt.Errorf("credential type not allowed: %q", credType)
	}
	return nil
}

// parseCredType returns the matching CredType for the JSON type string if
// it is in the list of publicly exposed types, otherwise Unknown.
func parseCredType(typeString string) CredType {
	ct := CredType(typeString)
	if knownTypes[ct] {
		return ct
	}
	return Unknown
}
