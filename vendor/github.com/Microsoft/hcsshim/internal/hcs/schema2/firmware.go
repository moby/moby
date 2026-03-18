package hcsschema

type FirmwareFile struct {
	// Parameters is an experimental/pre-release field. The field itself or its
	// behavior can change in future iterations of the schema. Avoid taking a hard
	// dependency on this field.
	Parameters []byte `json:"Parameters,omitempty"`
}
