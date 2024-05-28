// Code generated by go-swagger; DO NOT EDIT.

package volume

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// ListResponse VolumeListResponse
//
// # Volume list response
//
// swagger:model ListResponse
type ListResponse struct {

	// List of volumes
	Volumes []*Volume `json:"Volumes"`

	// Warnings that occurred when fetching the list of volumes.
	//
	// Example: []
	Warnings []string `json:"Warnings"`
}

// Validate validates this list response
func (m *ListResponse) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateVolumes(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ListResponse) validateVolumes(formats strfmt.Registry) error {
	if swag.IsZero(m.Volumes) { // not required
		return nil
	}

	for i := 0; i < len(m.Volumes); i++ {
		if swag.IsZero(m.Volumes[i]) { // not required
			continue
		}

		if m.Volumes[i] != nil {
			if err := m.Volumes[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("Volumes" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("Volumes" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this list response based on the context it is used
func (m *ListResponse) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateVolumes(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ListResponse) contextValidateVolumes(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.Volumes); i++ {

		if m.Volumes[i] != nil {

			if swag.IsZero(m.Volumes[i]) { // not required
				return nil
			}

			if err := m.Volumes[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("Volumes" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("Volumes" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *ListResponse) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ListResponse) UnmarshalBinary(b []byte) error {
	var res ListResponse
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
