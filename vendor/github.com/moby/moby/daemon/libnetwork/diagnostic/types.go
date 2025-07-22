package diagnostic

import "fmt"

// StringInterface interface that has to be implemented by messages
type StringInterface interface {
	String() string
}

// CommandSucceed creates a success message
func CommandSucceed(result StringInterface) *HTTPResult {
	return &HTTPResult{
		Message: "OK",
		Details: result,
	}
}

// FailCommand creates a failure message with error
func FailCommand(err error) *HTTPResult {
	return &HTTPResult{
		Message: "FAIL",
		Details: &ErrorCmd{Error: err.Error()},
	}
}

// WrongCommand creates a wrong command response
func WrongCommand(message, usage string) *HTTPResult {
	return &HTTPResult{
		Message: message,
		Details: &UsageCmd{Usage: usage},
	}
}

// HTTPResult Diagnostic Server HTTP result operation
type HTTPResult struct {
	Message string          `json:"message"`
	Details StringInterface `json:"details"`
}

func (h *HTTPResult) String() string {
	rsp := h.Message
	if h.Details != nil {
		rsp += "\n" + h.Details.String()
	}
	return rsp
}

// UsageCmd command with usage field
type UsageCmd struct {
	Usage string `json:"usage"`
}

func (u *UsageCmd) String() string {
	return "Usage: " + u.Usage
}

// StringCmd command with info string
type StringCmd struct {
	Info string `json:"info"`
}

func (s *StringCmd) String() string {
	return s.Info
}

// ErrorCmd command with error
type ErrorCmd struct {
	Error string `json:"error"`
}

func (e *ErrorCmd) String() string {
	return "Error: " + e.Error
}

// TableObj network db table object
type TableObj struct {
	Length   int               `json:"size"`
	Elements []StringInterface `json:"entries"`
}

func (t *TableObj) String() string {
	output := fmt.Sprintf("total entries: %d\n", t.Length)
	for _, e := range t.Elements {
		output += e.String()
	}
	return output
}

// PeerEntryObj entry in the networkdb peer table
type PeerEntryObj struct {
	Index int    `json:"-"`
	Name  string `json:"-=name"`
	IP    string `json:"ip"`
}

func (p *PeerEntryObj) String() string {
	return fmt.Sprintf("%d) %s -> %s\n", p.Index, p.Name, p.IP)
}

// TableEntryObj network db table entry object
type TableEntryObj struct {
	Index int    `json:"-"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Owner string `json:"owner"`
}

func (t *TableEntryObj) String() string {
	return fmt.Sprintf("%d) k:`%s` -> v:`%s` owner:`%s`\n", t.Index, t.Key, t.Value, t.Owner)
}

// TableEndpointsResult fully typed message for proper unmarshaling on the client side
type TableEndpointsResult struct {
	TableObj
	Elements []TableEntryObj `json:"entries"`
}

// TablePeersResult fully typed message for proper unmarshaling on the client side
type TablePeersResult struct {
	TableObj
	Elements []PeerEntryObj `json:"entries"`
}

// NetworkStatsResult network db stats related to entries and queue len for a network
type NetworkStatsResult struct {
	Entries  int `json:"entries"`
	QueueLen int `jsoin:"qlen"`
}

func (n *NetworkStatsResult) String() string {
	return fmt.Sprintf("entries: %d, qlen: %d\n", n.Entries, n.QueueLen)
}
