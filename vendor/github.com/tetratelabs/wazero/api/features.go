package api

import (
	"fmt"
	"strings"
)

// CoreFeatures is a bit flag of WebAssembly Core specification features. See
// https://github.com/WebAssembly/proposals for proposals and their status.
//
// Constants define individual features, such as CoreFeatureMultiValue, or
// groups of "finished" features, assigned to a WebAssembly Core Specification
// version, e.g. CoreFeaturesV1 or CoreFeaturesV2.
//
// Note: Numeric values are not intended to be interpreted except as bit flags.
type CoreFeatures uint64

// CoreFeaturesV1 are features included in the WebAssembly Core Specification
// 1.0. As of late 2022, this is the only version that is a Web Standard (W3C
// Recommendation).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/
const CoreFeaturesV1 = CoreFeatureMutableGlobal

// CoreFeaturesV2 are features included in the WebAssembly Core Specification
// 2.0 (20220419). As of late 2022, version 2.0 is a W3C working draft, not yet
// a Web Standard (W3C Recommendation).
//
// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#release-1-1
const CoreFeaturesV2 = CoreFeaturesV1 |
	CoreFeatureBulkMemoryOperations |
	CoreFeatureMultiValue |
	CoreFeatureNonTrappingFloatToIntConversion |
	CoreFeatureReferenceTypes |
	CoreFeatureSignExtensionOps |
	CoreFeatureSIMD

const (
	// CoreFeatureBulkMemoryOperations adds instructions modify ranges of
	// memory or table entries ("bulk-memory-operations"). This is included in
	// CoreFeaturesV2, but not CoreFeaturesV1.
	//
	// Here are the notable effects:
	//   - Adds `memory.fill`, `memory.init`, `memory.copy` and `data.drop`
	//     instructions.
	//   - Adds `table.init`, `table.copy` and `elem.drop` instructions.
	//   - Introduces a "passive" form of element and data segments.
	//   - Stops checking "active" element and data segment boundaries at
	//     compile-time, meaning they can error at runtime.
	//
	// Note: "bulk-memory-operations" is mixed with the "reference-types"
	// proposal due to the WebAssembly Working Group merging them
	// "mutually dependent". Therefore, enabling this feature requires enabling
	// CoreFeatureReferenceTypes, and vice-versa.
	//
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/bulk-memory-operations/Overview.md
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/reference-types/Overview.md and
	// https://github.com/WebAssembly/spec/pull/1287
	CoreFeatureBulkMemoryOperations CoreFeatures = 1 << iota

	// CoreFeatureMultiValue enables multiple values ("multi-value"). This is
	// included in CoreFeaturesV2, but not CoreFeaturesV1.
	//
	// Here are the notable effects:
	//   - Function (`func`) types allow more than one result.
	//   - Block types (`block`, `loop` and `if`) can be arbitrary function
	//     types.
	//
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/multi-value/Overview.md
	CoreFeatureMultiValue

	// CoreFeatureMutableGlobal allows globals to be mutable. This is included
	// in both CoreFeaturesV1 and CoreFeaturesV2.
	//
	// When false, an api.Global can never be cast to an api.MutableGlobal, and
	// any wasm that includes global vars will fail to parse.
	CoreFeatureMutableGlobal

	// CoreFeatureNonTrappingFloatToIntConversion enables non-trapping
	// float-to-int conversions ("nontrapping-float-to-int-conversion"). This
	// is included in CoreFeaturesV2, but not CoreFeaturesV1.
	//
	// The only effect of enabling is allowing the following instructions,
	// which return 0 on NaN instead of panicking.
	//   - `i32.trunc_sat_f32_s`
	//   - `i32.trunc_sat_f32_u`
	//   - `i32.trunc_sat_f64_s`
	//   - `i32.trunc_sat_f64_u`
	//   - `i64.trunc_sat_f32_s`
	//   - `i64.trunc_sat_f32_u`
	//   - `i64.trunc_sat_f64_s`
	//   - `i64.trunc_sat_f64_u`
	//
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/nontrapping-float-to-int-conversion/Overview.md
	CoreFeatureNonTrappingFloatToIntConversion

	// CoreFeatureReferenceTypes enables various instructions and features
	// related to table and new reference types. This is included in
	// CoreFeaturesV2, but not CoreFeaturesV1.
	//
	//   - Introduction of new value types: `funcref` and `externref`.
	//   - Support for the following new instructions:
	//     - `ref.null`
	//     - `ref.func`
	//     - `ref.is_null`
	//     - `table.fill`
	//     - `table.get`
	//     - `table.grow`
	//     - `table.set`
	//     - `table.size`
	//   - Support for multiple tables per module:
	//     - `call_indirect`, `table.init`, `table.copy` and `elem.drop`
	//   - Support for instructions can take non-zero table index.
	//     - Element segments can take non-zero table index.
	//
	// Note: "reference-types" is mixed with the "bulk-memory-operations"
	// proposal due to the WebAssembly Working Group merging them
	// "mutually dependent". Therefore, enabling this feature requires enabling
	// CoreFeatureBulkMemoryOperations, and vice-versa.
	//
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/bulk-memory-operations/Overview.md
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/reference-types/Overview.md and
	// https://github.com/WebAssembly/spec/pull/1287
	CoreFeatureReferenceTypes

	// CoreFeatureSignExtensionOps enables sign extension instructions
	// ("sign-extension-ops"). This is included in CoreFeaturesV2, but not
	// CoreFeaturesV1.
	//
	// Adds instructions:
	//   - `i32.extend8_s`
	//   - `i32.extend16_s`
	//   - `i64.extend8_s`
	//   - `i64.extend16_s`
	//   - `i64.extend32_s`
	//
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/sign-extension-ops/Overview.md
	CoreFeatureSignExtensionOps

	// CoreFeatureSIMD enables the vector value type and vector instructions
	// (aka SIMD). This is included in CoreFeaturesV2, but not CoreFeaturesV1.
	//
	// Note: The instruction list is too long to enumerate in godoc.
	// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md
	CoreFeatureSIMD

	// Update experimental/features.go when adding elements here.
)

// SetEnabled enables or disables the feature or group of features.
func (f CoreFeatures) SetEnabled(feature CoreFeatures, val bool) CoreFeatures {
	if val {
		return f | feature
	}
	return f &^ feature
}

// IsEnabled returns true if the feature (or group of features) is enabled.
func (f CoreFeatures) IsEnabled(feature CoreFeatures) bool {
	return f&feature != 0
}

// RequireEnabled returns an error if the feature (or group of features) is not
// enabled.
func (f CoreFeatures) RequireEnabled(feature CoreFeatures) error {
	if f&feature == 0 {
		return fmt.Errorf("feature %q is disabled", feature)
	}
	return nil
}

// String implements fmt.Stringer by returning each enabled feature.
func (f CoreFeatures) String() string {
	var builder strings.Builder
	for i := 0; i <= 63; i++ { // cycle through all bits to reduce code and maintenance
		target := CoreFeatures(1 << i)
		if f.IsEnabled(target) {
			if name := featureName(target); name != "" {
				if builder.Len() > 0 {
					builder.WriteByte('|')
				}
				builder.WriteString(name)
			}
		}
	}
	return builder.String()
}

func featureName(f CoreFeatures) string {
	switch f {
	case CoreFeatureMutableGlobal:
		// match https://github.com/WebAssembly/mutable-global
		return "mutable-global"
	case CoreFeatureSignExtensionOps:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/sign-extension-ops/Overview.md
		return "sign-extension-ops"
	case CoreFeatureMultiValue:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/multi-value/Overview.md
		return "multi-value"
	case CoreFeatureNonTrappingFloatToIntConversion:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/nontrapping-float-to-int-conversion/Overview.md
		return "nontrapping-float-to-int-conversion"
	case CoreFeatureBulkMemoryOperations:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/bulk-memory-operations/Overview.md
		return "bulk-memory-operations"
	case CoreFeatureReferenceTypes:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/reference-types/Overview.md
		return "reference-types"
	case CoreFeatureSIMD:
		// match https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md
		return "simd"
	}
	return ""
}
