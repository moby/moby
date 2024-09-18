package pb

import "encoding/json"

func (m *Op) UnmarshalJSON(data []byte) error {
	var v struct {
		Inputs []*Input `json:"inputs,omitempty"`
		Op     struct {
			*Op_Exec
			*Op_Source
			*Op_File
			*Op_Build
			*Op_Merge
			*Op_Diff
		}
		Platform    *Platform          `json:"platform,omitempty"`
		Constraints *WorkerConstraints `json:"constraints,omitempty"`
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	m.Inputs = v.Inputs
	switch {
	case v.Op.Op_Exec != nil:
		m.Op = v.Op.Op_Exec
	case v.Op.Op_Source != nil:
		m.Op = v.Op.Op_Source
	case v.Op.Op_File != nil:
		m.Op = v.Op.Op_File
	case v.Op.Op_Build != nil:
		m.Op = v.Op.Op_Build
	case v.Op.Op_Merge != nil:
		m.Op = v.Op.Op_Merge
	case v.Op.Op_Diff != nil:
		m.Op = v.Op.Op_Diff
	}
	m.Platform = v.Platform
	m.Constraints = v.Constraints
	return nil
}

func (m *FileAction) UnmarshalJSON(data []byte) error {
	var v struct {
		Input          InputIndex  `json:"input"`
		SecondaryInput InputIndex  `json:"secondaryInput"`
		Output         OutputIndex `json:"output"`
		Action         struct {
			*FileAction_Copy
			*FileAction_Mkfile
			*FileAction_Mkdir
			*FileAction_Rm
		}
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	m.Input = v.Input
	m.SecondaryInput = v.SecondaryInput
	m.Output = v.Output
	switch {
	case v.Action.FileAction_Copy != nil:
		m.Action = v.Action.FileAction_Copy
	case v.Action.FileAction_Mkfile != nil:
		m.Action = v.Action.FileAction_Mkfile
	case v.Action.FileAction_Mkdir != nil:
		m.Action = v.Action.FileAction_Mkdir
	case v.Action.FileAction_Rm != nil:
		m.Action = v.Action.FileAction_Rm
	}
	return nil
}

func (m *UserOpt) UnmarshalJSON(data []byte) error {
	var v struct {
		User struct {
			*UserOpt_ByName
			*UserOpt_ByID
		}
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch {
	case v.User.UserOpt_ByName != nil:
		m.User = v.User.UserOpt_ByName
	case v.User.UserOpt_ByID != nil:
		m.User = v.User.UserOpt_ByID
	}
	return nil
}
