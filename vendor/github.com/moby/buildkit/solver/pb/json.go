package pb

import "encoding/json"

type jsonOp struct {
	Inputs []*Input `json:"inputs,omitempty"`
	Op     struct {
		Exec   *ExecOp   `json:"exec,omitempty"`
		Source *SourceOp `json:"source,omitempty"`
		File   *FileOp   `json:"file,omitempty"`
		Build  *BuildOp  `json:"build,omitempty"`
		Merge  *MergeOp  `json:"merge,omitempty"`
		Diff   *DiffOp   `json:"diff,omitempty"`
	}
	Platform    *Platform          `json:"platform,omitempty"`
	Constraints *WorkerConstraints `json:"constraints,omitempty"`
}

func (m *Op) MarshalJSON() ([]byte, error) {
	var v jsonOp
	v.Inputs = m.Inputs
	switch op := m.Op.(type) {
	case *Op_Exec:
		v.Op.Exec = op.Exec
	case *Op_Source:
		v.Op.Source = op.Source
	case *Op_File:
		v.Op.File = op.File
	case *Op_Build:
		v.Op.Build = op.Build
	case *Op_Merge:
		v.Op.Merge = op.Merge
	case *Op_Diff:
		v.Op.Diff = op.Diff
	}
	v.Platform = m.Platform
	v.Constraints = m.Constraints
	return json.Marshal(v)
}

func (m *Op) UnmarshalJSON(data []byte) error {
	var v jsonOp
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	m.Inputs = v.Inputs
	switch {
	case v.Op.Exec != nil:
		m.Op = &Op_Exec{v.Op.Exec}
	case v.Op.Source != nil:
		m.Op = &Op_Source{v.Op.Source}
	case v.Op.File != nil:
		m.Op = &Op_File{v.Op.File}
	case v.Op.Build != nil:
		m.Op = &Op_Build{v.Op.Build}
	case v.Op.Merge != nil:
		m.Op = &Op_Merge{v.Op.Merge}
	case v.Op.Diff != nil:
		m.Op = &Op_Diff{v.Op.Diff}
	}
	m.Platform = v.Platform
	m.Constraints = v.Constraints
	return nil
}

type jsonFileAction struct {
	Input          InputIndex  `json:"input"`
	SecondaryInput InputIndex  `json:"secondaryInput"`
	Output         OutputIndex `json:"output"`
	Action         struct {
		Copy   *FileActionCopy   `json:"copy,omitempty"`
		Mkfile *FileActionMkFile `json:"mkfile,omitempty"`
		Mkdir  *FileActionMkDir  `json:"mkdir,omitempty"`
		Rm     *FileActionRm     `json:"rm,omitempty"`
	}
}

func (m *FileAction) MarshalJSON() ([]byte, error) {
	var v jsonFileAction
	v.Input = InputIndex(m.Input)
	v.SecondaryInput = InputIndex(m.SecondaryInput)
	v.Output = OutputIndex(m.Output)
	switch action := m.Action.(type) {
	case *FileAction_Copy:
		v.Action.Copy = action.Copy
	case *FileAction_Mkfile:
		v.Action.Mkfile = action.Mkfile
	case *FileAction_Mkdir:
		v.Action.Mkdir = action.Mkdir
	case *FileAction_Rm:
		v.Action.Rm = action.Rm
	}
	return json.Marshal(v)
}

func (m *FileAction) UnmarshalJSON(data []byte) error {
	var v jsonFileAction
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	m.Input = int64(v.Input)
	m.SecondaryInput = int64(v.SecondaryInput)
	m.Output = int64(v.Output)
	switch {
	case v.Action.Copy != nil:
		m.Action = &FileAction_Copy{v.Action.Copy}
	case v.Action.Mkfile != nil:
		m.Action = &FileAction_Mkfile{v.Action.Mkfile}
	case v.Action.Mkdir != nil:
		m.Action = &FileAction_Mkdir{v.Action.Mkdir}
	case v.Action.Rm != nil:
		m.Action = &FileAction_Rm{v.Action.Rm}
	}
	return nil
}

type jsonUserOpt struct {
	User struct {
		ByName *NamedUserOpt `json:"byName,omitempty"`
		ByID   uint32        `json:"byId,omitempty"`
	}
}

func (m *UserOpt) MarshalJSON() ([]byte, error) {
	var v jsonUserOpt
	switch userOpt := m.User.(type) {
	case *UserOpt_ByName:
		v.User.ByName = userOpt.ByName
	case *UserOpt_ByID:
		v.User.ByID = userOpt.ByID
	}
	return json.Marshal(v)
}

func (m *UserOpt) UnmarshalJSON(data []byte) error {
	var v jsonUserOpt
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch {
	case v.User.ByName != nil:
		m.User = &UserOpt_ByName{v.User.ByName}
	default:
		m.User = &UserOpt_ByID{v.User.ByID}
	}
	return nil
}
