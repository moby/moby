package pb

// InputIndex is incrementing index to the input vertex
type InputIndex int64

// OutputIndex is incrementing index that another vertex can depend on
type OutputIndex int64

// RootMount is a base mountpoint
const RootMount = "/"

// SkipOutput marks a disabled output index
const SkipOutput OutputIndex = -1

// Empty marks an input with no content
const Empty InputIndex = -1

// LLBBuilder is a special builder for BuildOp that directly builds LLB
const LLBBuilder InputIndex = -1

// LLBDefinitionInput marks an input that contains LLB definition for BuildOp
const LLBDefinitionInput = "buildkit.llb.definition"

// LLBDefaultDefinitionFile is a filename containing the definition in LLBBuilder
const LLBDefaultDefinitionFile = LLBDefinitionInput
