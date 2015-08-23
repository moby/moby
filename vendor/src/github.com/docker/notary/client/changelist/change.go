package changelist

// Scopes for TufChanges are simply the TUF roles.
// Unfortunately because of targets delegations, we can only
// cover the base roles.
const (
	ScopeRoot      = "root"
	ScopeTargets   = "targets"
	ScopeSnapshot  = "snapshot"
	ScopeTimestamp = "timestamp"
)

// TufChange represents a change to a TUF repo
type TufChange struct {
	// Abbreviated because Go doesn't permit a field and method of the same name
	Actn       string `json:"action"`
	Role       string `json:"role"`
	ChangeType string `json:"type"`
	ChangePath string `json:"path"`
	Data       []byte `json:"data"`
}

// NewTufChange initializes a tufChange object
func NewTufChange(action string, role, changeType, changePath string, content []byte) *TufChange {
	return &TufChange{
		Actn:       action,
		Role:       role,
		ChangeType: changeType,
		ChangePath: changePath,
		Data:       content,
	}
}

// Action return c.Actn
func (c TufChange) Action() string {
	return c.Actn
}

// Scope returns c.Role
func (c TufChange) Scope() string {
	return c.Role
}

// Type returns c.ChangeType
func (c TufChange) Type() string {
	return c.ChangeType
}

// Path return c.ChangePath
func (c TufChange) Path() string {
	return c.ChangePath
}

// Content returns c.Data
func (c TufChange) Content() []byte {
	return c.Data
}
