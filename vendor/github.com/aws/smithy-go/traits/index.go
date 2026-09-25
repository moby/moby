package traits

// Trait index constants, ordered by frequency of occurrence across AWS API
// models. Lower indices are assigned to more common traits so that the
// per-schema indexed slice stays small.
const (
	indexJSONName = iota
	indexHTTP
	indexHTTPLabel
	indexXMLName
	indexHTTPQuery
	indexEC2QueryName
	indexHTTPError
	indexHTTPHeader
	indexSensitive
	indexAWSQueryError
	indexTimestampFormat
	indexHTTPPayload
	indexContextParam
	indexHTTPResponseCode
	indexHostLabel
	indexXMLNamespace
	indexXMLFlattened
	indexStreaming
	indexMediaType
	indexHTTPQueryParams
	indexEventPayload
	indexHTTPPrefixHeaders
	indexEventHeader
	indexXMLAttribute
	indexUnitShape
)

// TraitIndex implements [smithy.IndexableTrait].
func (*JSONName) TraitIndex() int { return indexJSONName }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTP) TraitIndex() int { return indexHTTP }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPLabel) TraitIndex() int { return indexHTTPLabel }

// TraitIndex implements [smithy.IndexableTrait].
func (*XMLName) TraitIndex() int { return indexXMLName }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPQuery) TraitIndex() int { return indexHTTPQuery }

// TraitIndex implements [smithy.IndexableTrait].
func (*EC2QueryName) TraitIndex() int { return indexEC2QueryName }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPError) TraitIndex() int { return indexHTTPError }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPHeader) TraitIndex() int { return indexHTTPHeader }

// TraitIndex implements [smithy.IndexableTrait].
func (*Sensitive) TraitIndex() int { return indexSensitive }

// TraitIndex implements [smithy.IndexableTrait].
func (*AWSQueryError) TraitIndex() int { return indexAWSQueryError }

// TraitIndex implements [smithy.IndexableTrait].
func (*TimestampFormat) TraitIndex() int { return indexTimestampFormat }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPPayload) TraitIndex() int { return indexHTTPPayload }

// TraitIndex implements [smithy.IndexableTrait].
func (*ContextParam) TraitIndex() int { return indexContextParam }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPResponseCode) TraitIndex() int { return indexHTTPResponseCode }

// TraitIndex implements [smithy.IndexableTrait].
func (*HostLabel) TraitIndex() int { return indexHostLabel }

// TraitIndex implements [smithy.IndexableTrait].
func (*XMLNamespace) TraitIndex() int { return indexXMLNamespace }

// TraitIndex implements [smithy.IndexableTrait].
func (*XMLFlattened) TraitIndex() int { return indexXMLFlattened }

// TraitIndex implements [smithy.IndexableTrait].
func (*Streaming) TraitIndex() int { return indexStreaming }

// TraitIndex implements [smithy.IndexableTrait].
func (*MediaType) TraitIndex() int { return indexMediaType }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPQueryParams) TraitIndex() int { return indexHTTPQueryParams }

// TraitIndex implements [smithy.IndexableTrait].
func (*EventPayload) TraitIndex() int { return indexEventPayload }

// TraitIndex implements [smithy.IndexableTrait].
func (*HTTPPrefixHeaders) TraitIndex() int { return indexHTTPPrefixHeaders }

// TraitIndex implements [smithy.IndexableTrait].
func (*EventHeader) TraitIndex() int { return indexEventHeader }

// TraitIndex implements [smithy.IndexableTrait].
func (*XMLAttribute) TraitIndex() int { return indexXMLAttribute }

// TraitIndex implements [smithy.IndexableTrait].
func (*UnitShape) TraitIndex() int { return indexUnitShape }
