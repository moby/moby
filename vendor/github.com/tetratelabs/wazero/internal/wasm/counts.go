package wasm

import "fmt"

// SectionElementCount returns the count of elements in a given section ID
//
// For example...
// * SectionIDType returns the count of FunctionType
// * SectionIDCustom returns the count of CustomSections plus one if NameSection is present
// * SectionIDHostFunction returns the count of HostFunctionSection
// * SectionIDExport returns the count of unique export names
func (m *Module) SectionElementCount(sectionID SectionID) uint32 { // element as in vector elements!
	switch sectionID {
	case SectionIDCustom:
		numCustomSections := uint32(len(m.CustomSections))
		if m.NameSection != nil {
			numCustomSections++
		}
		return numCustomSections
	case SectionIDType:
		return uint32(len(m.TypeSection))
	case SectionIDImport:
		return uint32(len(m.ImportSection))
	case SectionIDFunction:
		return uint32(len(m.FunctionSection))
	case SectionIDTable:
		return uint32(len(m.TableSection))
	case SectionIDMemory:
		if m.MemorySection != nil {
			return 1
		}
		return 0
	case SectionIDGlobal:
		return uint32(len(m.GlobalSection))
	case SectionIDExport:
		return uint32(len(m.ExportSection))
	case SectionIDStart:
		if m.StartSection != nil {
			return 1
		}
		return 0
	case SectionIDElement:
		return uint32(len(m.ElementSection))
	case SectionIDCode:
		return uint32(len(m.CodeSection))
	case SectionIDData:
		return uint32(len(m.DataSection))
	default:
		panic(fmt.Errorf("BUG: unknown section: %d", sectionID))
	}
}
